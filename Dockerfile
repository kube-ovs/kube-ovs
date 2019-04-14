FROM ubuntu:16.04

RUN apt update && apt install -y openvswitch-switch iptables

ADD kube-ovs /bin
ADD kube-ovs-cni /bin

CMD ["/bin/kube-ovs"]
