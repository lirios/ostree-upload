FROM alpine:3.11 AS build
RUN mkdir /source
ADD . /source/
WORKDIR /source
RUN set -ex && \
    mkdir /build && \
    cp /source/target/release/ostree-upload /build/ && \
    cp /source/target/release/ostree-receive /build/

FROM alpine:3.11
COPY --from=build /build/ostree-upload /usr/bin
COPY --from=build /build/ostree-receive /usr/bin
RUN apk add ostree openssl
ENTRYPOINT ["/usr/bin/ostree-upload"]
