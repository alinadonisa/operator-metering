FROM quay.io/coreos/metering-builder:latest as build

RUN mkdir -p /out
VOLUME /out

ENV HELM_VERSION 2.8.0
RUN curl \
    --silent \
    --show-error \
    --location \
    "https://storage.googleapis.com/kubernetes-helm/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
    | tar xz --strip-components=1 -C /usr/local/bin linux-amd64/helm \
    && chmod +x /usr/local/bin/helm

env KUBERNETES_VERSION 1.8.3
RUN curl \
    --silent \
    --show-error \
    --location \
    "https://storage.googleapis.com/kubernetes-release/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubectl" \
    -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

ENV STERN_VERSION 1.6.0
RUN curl \
    --silent \
    --show-error \
    --location \
    "https://github.com/wercker/stern/releases/download/${STERN_VERSION}/stern_linux_amd64" \
    -o /usr/local/bin/stern \
    && chmod +x /usr/local/bin/stern

RUN helm init --client-only --skip-refresh && helm repo remove stable || true

COPY . $GOPATH/src/github.com/operator-framework/operator-metering
WORKDIR $GOPATH/src/github.com/operator-framework/operator-metering

RUN cd gotools/test2json && go build -o ../../bin/test2json main.go

CMD ["hack/e2e.sh"]
