FROM quay.io/joelanford/helm-operator:master

ENV HOME=/opt/helm
COPY --chown=65532 watches.yaml ${HOME}/watches.yaml
COPY --chown=65532 helm-charts  ${HOME}/helm-charts
COPY --chown=65532 PROJECT  ${HOME}/PROJECT
WORKDIR ${HOME}
