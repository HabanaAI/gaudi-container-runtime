FROM golang:1.25.3 AS checks
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
COPY habana-container-runtime /usr/bin/habana-container-runtime
COPY habana-container-hook /usr/bin/habana-container-hook
COPY habana-container-cli /usr/bin/habana-container-cli
COPY habana-container-toolkit /usr/bin/habana-container-toolkit
RUN chmod 0755 /usr/bin/habana-container-runtime /usr/bin/habana-container-hook /usr/bin/habana-container-cli /usr/bin/habana-container-toolkit

COPY packaging/config.toml /etc/habana-container-runtime/config.toml
COPY packaging/oci-habana-hook /usr/libexec/oci/hooks.d/oci-habana-hook
COPY packaging/oci-habana-hook.json /usr/share/containers/oci/hooks.d/oci-habana-hook.json
COPY LICENSE /usr/share/doc/habana-container-runtime/copyright
RUN chmod 0644 /etc/habana-container-runtime/config.toml /usr/libexec/oci/hooks.d/oci-habana-hook /usr/share/containers/oci/hooks.d/oci-habana-hook.json /usr/share/doc/habana-container-runtime/copyright

# Verify binary build specs with checksec
ENV CHECKSEC_REF="No RELRO,No Canary found,NX enabled,No PIE,N/A,N/A,No Symbols,N/A,0,0"
RUN apt-get update -y && apt-get --no-install-recommends -y install file && \
    wget -q https://raw.githubusercontent.com/slimm609/checksec/refs/heads/main/checksec.bash -O checksec && \
    chmod +x checksec
RUN ./checksec --file=/usr/bin/habana-container-cli --output=csv | grep -q "$CHECKSEC_REF"
RUN ./checksec --file=/usr/bin/habana-container-hook --output=csv | grep -q "$CHECKSEC_REF"
RUN ./checksec --file=/usr/bin/habana-container-runtime --output=csv | grep -q "$CHECKSEC_REF"
RUN ./checksec --file=/usr/bin/habana-container-toolkit --output=csv | grep -q "$CHECKSEC_REF"


FROM busybox:1.37.0
COPY --from=checks /usr/bin/habana-container-runtime /usr/bin/habana-container-runtime
COPY --from=checks /usr/bin/habana-container-hook /usr/bin/habana-container-hook
COPY --from=checks /usr/bin/habana-container-cli /usr/bin/habana-container-cli
COPY --from=checks /usr/bin/habana-container-toolkit /usr/bin/habana-container-toolkit

COPY --from=checks /etc/habana-container-runtime/config.toml /etc/habana-container-runtime/config.toml
COPY --from=checks /usr/libexec/oci/hooks.d/oci-habana-hook /usr/libexec/oci/hooks.d/oci-habana-hook
COPY --from=checks /usr/share/containers/oci/hooks.d/oci-habana-hook.json /usr/share/containers/oci/hooks.d/oci-habana-hook.json
COPY --from=checks /usr/share/doc/habana-container-runtime/copyright /usr/share/doc/habana-container-runtime/copyright

COPY licenses/ /licenses
COPY LICENSE /licenses/intel-gaudi-container-runtime/LICENSE

USER 65532:65532
