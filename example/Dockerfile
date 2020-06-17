# only the makefile and dockerfile  are the only scaffods that need to be updated from the base
FROM quay.io/joelanford/helm-operator:latest

ENV HOME=/opt/helm
COPY watches.yaml ${HOME}/watches.yaml
COPY helm-charts  ${HOME}/helm-charts
WORKDIR ${HOME}
