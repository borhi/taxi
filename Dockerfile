FROM golang:1.14-alpine as build
WORKDIR /src/taxi
COPY . .
RUN go build

FROM alpine:latest
COPY --from=build /src/taxi .
CMD ["./taxi", "-port", "3000"]
EXPOSE 3000
