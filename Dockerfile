FROM golang:1.19 AS builder

WORKDIR /go/src/github.com/fi-ts/gardener-extension-authn
COPY . .
RUN make install

FROM alpine:3.17
WORKDIR /
COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-authn /gardener-extension-authn
CMD ["/gardener-extension-authn"]
