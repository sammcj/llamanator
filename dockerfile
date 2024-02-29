FROM golang:1.21 as builder
WORKDIR /app
COPY . .
RUN go build -o build/llamanator

FROM gcr.io/distroless/base
WORKDIR /app

RUN useradd -u 3011 appuser && chown -R appuser /app
COPY --chown=appuser:appuser --from=builder /app/llamanator .
USER appuser

# Can mount in allowlist etc here
VOLUME [ "/config" ]

EXPOSE 8080
CMD ["./llamanator"]
