FROM scratch
COPY github-flow-manager /
ENTRYPOINT ["/github-flow-manager"]