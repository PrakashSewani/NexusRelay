FROM busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a AS busybox

FROM arigaio/atlas:1.2.3-community@sha256:d43f072e79dd98554b890fb56d159b14c0a4ccdb8609296da16402d015483cfa

ARG VERSION
ARG REVISION

WORKDIR /workspace

COPY --from=busybox /bin/busybox /bin/busybox
RUN ["/bin/busybox", "sh", "-c", "case \"$VERSION\" in \"\"|*[!A-Za-z0-9._+-]*) echo \"VERSION must match [A-Za-z0-9._+-]+\" >&2; exit 1;; esac; case \"$REVISION\" in [A-Za-z0-9]*);; *) echo \"REVISION must start with an ASCII letter or number\" >&2; exit 1;; esac; case \"$REVISION\" in *[!A-Za-z0-9._-]*) echo \"REVISION must match [A-Za-z0-9][A-Za-z0-9._-]{0,127}\" >&2; exit 1;; esac; test \"${#REVISION}\" -le 128 || { echo \"REVISION must be at most 128 characters\" >&2; exit 1; }"]

LABEL org.opencontainers.image.title="NexusRelay migrations" \
      org.opencontainers.image.description="Pinned Atlas Community migration runner for NexusRelay" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/PrakashSewani/NexusRelay"

COPY --chown=65532:65532 atlas.hcl /workspace/atlas.hcl
COPY --chown=65532:65532 migrations /workspace/migrations
COPY --chown=65532:65532 --chmod=0555 deploy/migrate/entrypoint.sh /entrypoint.sh

USER 65532:65532

ENTRYPOINT ["/entrypoint.sh"]
