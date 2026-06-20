package classify

// IsGroupMessage reports whether the headers look like mailing-list / Google-
// Group traffic (Spec 0008): an X-Google-Group-Id, or a List-Id together with
// a List-Post or List-Help header.
func IsGroupMessage(h Headers) bool {
	if h.XGoogleGroupID != "" {
		return true
	}
	return h.ListID != "" && (h.ListPost != "" || h.ListHelp != "")
}
