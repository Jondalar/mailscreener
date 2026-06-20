package imap

import (
	"bufio"
	"bytes"
	"context"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// headerFields are the only header fields fetched for classification.
var headerFields = []string{
	"From", "Sender", "Return-Path", "Message-Id", "Subject",
	"List-Id", "List-Unsubscribe", "List-Post", "List-Help", "X-Google-Group-Id",
}

// Client is the go-imap/v2 implementation of Backend, plus IDLE support.
type Client struct {
	c       *imapclient.Client
	updates chan struct{}
}

// Dial connects to the IMAP server over TLS and logs in.
func Dial(addr, user, pass string) (*Client, error) {
	cl := &Client{updates: make(chan struct{}, 1)}
	opts := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(*imapclient.UnilateralDataMailbox) { cl.signal() },
		},
	}
	c, err := imapclient.DialTLS(addr, opts)
	if err != nil {
		return nil, err
	}
	if err := c.Login(user, pass).Wait(); err != nil {
		c.Close()
		return nil, err
	}
	cl.c = c
	return cl, nil
}

// Close logs out and closes the connection.
func (cl *Client) Close() error {
	cl.c.Logout().Wait()
	return cl.c.Close()
}

func (cl *Client) signal() {
	select {
	case cl.updates <- struct{}{}:
	default:
	}
}

// EnsureFolders creates and subscribes the given mailboxes (idempotent).
func (cl *Client) EnsureFolders(names []string) error {
	for _, n := range names {
		cl.c.Create(n, nil).Wait() // ALREADYEXISTS is fine
		cl.c.Subscribe(n).Wait()   // best-effort
	}
	return nil
}

func (cl *Client) selectMailbox(name string) (*imap.SelectData, error) {
	return cl.c.Select(name, nil).Wait()
}

// List fetches messages (with headers) from a folder.
func (cl *Client) List(folder string, unseenOnly bool) ([]Msg, error) {
	sd, err := cl.selectMailbox(folder)
	if err != nil {
		return nil, err
	}
	if sd.NumMessages == 0 {
		return nil, nil
	}
	var ss imap.SeqSet
	ss.AddRange(1, sd.NumMessages)
	return cl.fetch(folder, ss, unseenOnly)
}

func (cl *Client) fetch(folder string, numSet imap.NumSet, unseenOnly bool) ([]Msg, error) {
	opts := &imap.FetchOptions{
		UID:          true,
		Flags:        true,
		InternalDate: true,
		BodySection: []*imap.FetchItemBodySection{{
			Specifier:    imap.PartSpecifierHeader,
			HeaderFields: headerFields,
			Peek:         true,
		}},
	}
	bufs, err := cl.c.Fetch(numSet, opts).Collect()
	if err != nil {
		return nil, err
	}
	var out []Msg
	for _, b := range bufs {
		if unseenOnly && hasFlag(b.Flags, imap.FlagSeen) {
			continue
		}
		out = append(out, msgFromBuffer(folder, b))
	}
	return out, nil
}

// ListSince returns messages whose UID is greater than sinceUID.
func (cl *Client) ListSince(folder string, sinceUID uint32) ([]Msg, error) {
	sd, err := cl.selectMailbox(folder)
	if err != nil {
		return nil, err
	}
	if sd.NumMessages == 0 {
		return nil, nil
	}
	var us imap.UIDSet
	us.AddRange(imap.UID(sinceUID+1), 0) // (since+1):*  (Stop 0 == '*')
	msgs, err := cl.fetch(folder, us, false)
	if err != nil {
		return nil, err
	}
	// IMAP's "n:*" can echo the last message when n > max UID; drop anything
	// not actually newer than the watermark.
	out := msgs[:0]
	for _, m := range msgs {
		if m.UID > sinceUID {
			out = append(out, m)
		}
	}
	return out, nil
}

// ListOlderThan returns messages with INTERNALDATE before now-d.
func (cl *Client) ListOlderThan(folder string, d time.Duration) ([]Msg, error) {
	msgs, err := cl.List(folder, false)
	if err != nil {
		return nil, err
	}
	cut := time.Now().Add(-d)
	var out []Msg
	for _, m := range msgs {
		if !m.Date.IsZero() && m.Date.Before(cut) {
			out = append(out, m)
		}
	}
	return out, nil
}

// ListChildren returns child mailboxes under parent (e.g. "Snoozed/1w").
func (cl *Client) ListChildren(parent string) ([]string, error) {
	datas, err := cl.c.List("", parent+"/*", nil).Collect()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range datas {
		if d.Mailbox != "" && d.Mailbox != parent {
			out = append(out, d.Mailbox)
		}
	}
	return out, nil
}

// Move moves messages by UID to dest.
func (cl *Client) Move(folder string, uids []uint32, dest string) (map[uint32]uint32, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	if _, err := cl.selectMailbox(folder); err != nil {
		return nil, err
	}
	data, err := cl.c.Move(toUIDSet(uids), dest).Wait()
	if err != nil {
		return nil, err
	}
	// Map each source UID to its new dest UID from the COPYUID response. The two
	// sets correspond in order (RFC 4315); some servers omit them (empty map).
	mapping := map[uint32]uint32{}
	if data != nil {
		src, okS := uidNums(data.SourceUIDs)
		dst, okD := uidNums(data.DestUIDs)
		if okS && okD && len(src) == len(dst) {
			for i := range src {
				mapping[uint32(src[i])] = uint32(dst[i])
			}
		}
	}
	return mapping, nil
}

// uidNums expands a NumSet to its UIDs when it is a static UIDSet.
func uidNums(ns imap.NumSet) ([]imap.UID, bool) {
	us, ok := ns.(imap.UIDSet)
	if !ok {
		return nil, false
	}
	return us.Nums()
}

// MarkSeen sets the \Seen flag.
func (cl *Client) MarkSeen(folder string, uids []uint32) error {
	return cl.store(folder, uids, imap.StoreFlagsAdd)
}

// ClearSeen removes the \Seen flag.
func (cl *Client) ClearSeen(folder string, uids []uint32) error {
	return cl.store(folder, uids, imap.StoreFlagsDel)
}

func (cl *Client) store(folder string, uids []uint32, op imap.StoreFlagsOp) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := cl.selectMailbox(folder); err != nil {
		return err
	}
	store := &imap.StoreFlags{Op: op, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
	return cl.c.Store(toUIDSet(uids), store, nil).Close()
}

// Idle selects folder and waits for a mailbox change on it, an external wake
// signal, or the timeout (whichever first). It returns nil whether woken by a
// new message or the timeout; the caller then runs a sweep. extra may be nil
// (a nil channel blocks forever, so it is simply ignored).
func (cl *Client) Idle(ctx context.Context, folder string, timeout time.Duration, extra <-chan struct{}) error {
	if _, err := cl.selectMailbox(folder); err != nil {
		return err
	}
	cmd, err := cl.c.Idle()
	if err != nil {
		return err
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-cl.updates:
	case <-extra:
	case <-t.C:
	}
	if err := cmd.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

// --- helpers ---

func toUIDSet(uids []uint32) imap.UIDSet {
	ids := make([]imap.UID, len(uids))
	for i, u := range uids {
		ids[i] = imap.UID(u)
	}
	return imap.UIDSetNum(ids...)
}

func hasFlag(flags []imap.Flag, want imap.Flag) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func msgFromBuffer(folder string, b *imapclient.FetchMessageBuffer) Msg {
	m := Msg{UID: uint32(b.UID), Folder: folder, Date: b.InternalDate}
	for _, f := range b.Flags {
		m.Flags = append(m.Flags, string(f))
	}
	var hdr textproto.MIMEHeader
	if len(b.BodySection) > 0 {
		hdr = parseHeader(b.BodySection[0].Bytes)
	}
	m.Sender = parseSender(hdr)
	m.MID = stripAngle(hdr.Get("Message-Id"))
	m.Headers = classify.Headers{
		ListID:          strings.TrimSpace(hdr.Get("List-Id")),
		ListUnsubscribe: strings.TrimSpace(hdr.Get("List-Unsubscribe")),
		ListPost:        strings.TrimSpace(hdr.Get("List-Post")),
		ListHelp:        strings.TrimSpace(hdr.Get("List-Help")),
		XGoogleGroupID:  strings.TrimSpace(hdr.Get("X-Google-Group-Id")),
		Subject:         strings.TrimSpace(hdr.Get("Subject")),
	}
	return m
}

func parseHeader(raw []byte) textproto.MIMEHeader {
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(raw)))
	h, _ := r.ReadMIMEHeader()
	return h
}

// parseSender picks the first usable address from From, Sender, Return-Path.
func parseSender(h textproto.MIMEHeader) string {
	for _, field := range []string{"From", "Sender", "Return-Path"} {
		v := h.Get(field)
		if v == "" {
			continue
		}
		if addr, err := mail.ParseAddress(v); err == nil {
			return strings.ToLower(strings.TrimSpace(addr.Address))
		}
		// Fallback: strip <...>.
		if a := stripAngle(v); strings.Contains(a, "@") {
			return strings.ToLower(a)
		}
	}
	return ""
}

func stripAngle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '<'); i >= 0 {
		if j := strings.IndexByte(s[i:], '>'); j >= 0 {
			return strings.TrimSpace(s[i+1 : i+j])
		}
	}
	return strings.Trim(s, "<>")
}
