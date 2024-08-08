FROM golang:1.22-bookworm AS build
WORKDIR /app

COPY . .

RUN go mod tidy

RUN make build

FROM scratch
WORKDIR /app

COPY --from=build /app/public ./public
COPY --from=build /app/views ./views
COPY --from=build /app/bin .

EXPOSE 4000

CMD [ "/app/online_sandbox" ]
