FROM alpine:3.8

ADD kube-ovs /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/kube-ovs"]
