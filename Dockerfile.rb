# Build stage using specific images from stagex
FROM scratch AS build
COPY --from=stagex/pallet-go@sha256:da4d8eec91b2f34bb5cf4946f64fd46c4d914691435fa83dc6d777993e50de62 . /
COPY --from=stagex/gcc:13.1.0@sha256:439bf36289ef036a934129d69dd6b4c196427e4f8e28bc1a3de5b9aab6e062f0 . /
COPY --from=stagex/binutils:2.43.1@sha256:30a1bd110273894fe91c3a4a2103894f53eaac43cf12a035008a6982cb0e6908 . /
COPY --from=stagex/libunwind:1.7.2@sha256:97ee6068a8e8c9f1c74409f80681069c8051abb31f9559dedf0d0d562d3bfc82 . /
COPY --from=stagex/musl:1.2.4@sha256:ad351b875f26294562d21740a3ee51c23609f15e6f9f0310e0994179c4231e1d . /
COPY --from=stagex/llvm:18.1.8@sha256:30517a41af648305afe6398af5b8c527d25545037df9d977018c657ba1b1708f . /
COPY --from=stagex/zlib:1.3.1@sha256:96b4100550760026065dac57148d99e20a03d17e5ee20d6b32cbacd61125dbb6 . /
COPY --from=stagex/openssl:latest@sha256:8e3eb24b4d21639f7ea204b89211d8bc03a2e1b729fb1123f8d0b3752b4beaa1 . /

# Set environment variables for reproducibility
ENV SOURCE_DATE_EPOCH=1
ENV KBUILD_BUILD_TIMESTAMP=1
ENV OPENSSL_STATIC=true
ENV TZ=UTC
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV GOFLAGS="-mod=readonly"
ENV GOPROXY=https://proxy.golang.org,direct

WORKDIR /app

# Copy Go files and source
ADD go.mod go.mod
ADD go.sum go.sum
ADD erigon-lib/go.mod erigon-lib/go.mod
ADD erigon-lib/go.sum erigon-lib/go.sum

# Download dependencies and normalize timestamps
RUN mkdir -p /go/pkg && \
    go mod download && \
    find /go/pkg -exec touch -hcd "@${SOURCE_DATE_EPOCH}" {} \;

# Copy source code
ADD . .

# Build the Go application with deterministic settings
RUN --network=none GOOS=linux GOARCH=amd64 go build -a -trimpath \
    -ldflags='-extldflags "-static" -s -w -buildid=' \
    -tags nosqlite,noboltdb,nosilkworm \
    ./cmd/cdk-erigon && \
    mv cdk-erigon / && \
    # Normalize timestamp on the binary
    touch -hcd "@${SOURCE_DATE_EPOCH}" /cdk-erigon && \
    # Strip any unstable information from binary
    strip -s /cdk-erigon 2>/dev/null || true && \
    # Double-check all files have normalized timestamps
    find /app -type f -exec touch -hcd "@${SOURCE_DATE_EPOCH}" {} \; && \
    find /cdk-erigon -exec touch -hcd "@${SOURCE_DATE_EPOCH}" {} \;

RUN sha256sum /cdk-erigon > /cdk-erigon.sha256 && cat /cdk-erigon.sha256

# Runtime stage - minimal scratch image
FROM scratch

# Copy the compiled binary
COPY --from=build /cdk-erigon /cdk-erigon

# Set the entry point
ENTRYPOINT ["/cdk-erigon"]