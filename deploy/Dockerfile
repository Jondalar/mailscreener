# Deploy image: wraps a prebuilt static linux/amd64 screenerd binary. No Go
# toolchain needed on the NAS — only a distroless base + COPY.
FROM gcr.io/distroless/static-debian12:nonroot
COPY screenerd /screenerd
VOLUME ["/state"]
ENTRYPOINT ["/screenerd"]
