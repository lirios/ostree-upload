FROM rust:1.40-slim AS build
RUN mkdir /source
Add . /source/
WORKDIR /source
RUN set -ex && \
    apt-get update && \
    apt-get install -y libostree-dev libssl-dev && \
    cargo build --release && \
    strip target/release/ostree-upload && \
    strip target/release/ostree-receive && \
    mkdir /build && \
    cp target/release/ostree-upload /build/ && \
    cp target/release/ostree-receive /build/

FROM alpine:3.11
COPY --from=build /build/ostree-upload /usr/bin
COPY --from=build /build/ostree-receive /usr/bin
ENTRYPOINT ["/usr/bin/ostree-upload"]
