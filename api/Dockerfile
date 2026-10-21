FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -tags nodynamic -o /out/blogd ./cmd/blogd \
 && mkdir -p /out/data/media

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/blogd /usr/local/bin/blogd
COPY --from=build --chown=65532:65532 /out/data /data
ENV BLOG_LISTEN_ADDR=:8080 \
    BLOG_MEDIA_DIR=/data/media
VOLUME /data/media
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD ["blogd", "healthcheck"]
ENTRYPOINT ["blogd"]
CMD ["serve"]
