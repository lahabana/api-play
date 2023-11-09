FROM gcr.io/distroless/static-debian12

COPY LICENSE.txt \
    /build/artifacts-linux-${ARCH}/kuma-cp/kuma-cp /usr/bin

SHELL ["/busybox/busybox", "sh", "-c"]

