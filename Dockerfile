FROM golang:1.22 AS builder

WORKDIR /go/src/github.com/fi-ts/gardener-extension-authn
COPY . .
RUN make install \
 && strip /go/bin/gardener-extension-authn

FROM alpine:3.19
WORKDIR /
COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-authn /gardener-extension-authn
CMD ["/gardener-extension-authn"]
