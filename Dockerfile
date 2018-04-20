FROM alpine:3.6
WORKDIR /
COPY cacert.pem /etc/ssl/certs/ca-bundle.crt
# Now just add the binary
COPY app /
ENTRYPOINT ["/app"]
