FROM golang:1.17 as builder
ARG GITHUB_TOKEN
COPY . /app
WORKDIR /app
RUN hack/private-login.sh
RUN make build

FROM debian:stable-slim
COPY --from=builder /app/dsmcert /dsmcert
ENTRYPOINT ["/dsmcert"]
