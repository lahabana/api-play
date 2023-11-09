FROM gcr.io/distroless/static-debian12
ARG TARGETPLATFORM

WORKDIR /
COPY build/$TARGETPLATFORM/api-play /usr/bin
CMD chmod u+x /usr/bin/api-play

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/usr/bin/api-play"]
