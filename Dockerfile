# SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
#
# SPDX-License-Identifier: CC0-1.0

FROM golang:alpine AS build
RUN mkdir /source
COPY . /source/
WORKDIR /source
RUN set -ex && \
    apk --no-cache add ca-certificates build-base make git ostree-dev && \
    go mod download && \
    make && \
    strip bin/ostree-upload && \
    mkdir /build && \
    cp bin/ostree-upload /build/

FROM alpine
COPY --from=build /build/ostree-upload /usr/bin/ostree-upload
RUN apk --no-cache add libc6-compat ostree
ENTRYPOINT ["/usr/bin/ostree-upload"]
CMD ["--help"]
