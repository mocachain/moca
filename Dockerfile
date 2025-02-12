FROM golang:1.22.4-bullseye AS builder

ENV CGO_CFLAGS="-O -D__BLST_PORTABLE__"
ENV CGO_CFLAGS_ALLOW="-O -D__BLST_PORTABLE__"

ENV GOPRIVATE=github.com/zkMeLabs

ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:@github.com/".insteadOf "https://github.com/"

WORKDIR /workspace

COPY . .
RUN make build


FROM golang:1.22.4-bullseye

RUN apt-get update -y && apt-get install ca-certificates jq -y

WORKDIR /root

COPY --from=builder /workspace/build/mechaind /usr/bin/mechaind

CMD ["mechaind"]