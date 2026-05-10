FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -tags nodynamic -o /out/blog-api ./cmd/blog-api \
 && mkdir -p /out/data/media

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/blog-api /usr/local/bin/blog-api
COPY --from=build --chown=65532:65532 /out/data /data
ENV BLOG_LISTEN_ADDR=:8080 \
    BLOG_MEDIA_DIR=/data/media \
    BLOG_TRUST_PROXY=true
VOLUME /data/media
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD ["blog-api", "healthcheck"]
ENTRYPOINT ["blog-api"]
CMD ["serve"]
