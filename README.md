# maglev0

Faux-maglev-style hashing for managing VIPs using Iptables CLUSTERIP and
Zookeeper. Not sanctioned or supported by Google in any way.

[![Build Status](https://travis-ci.org/solarkennedy/maglev0.svg?branch=master)](https://travis-ci.org/solarkennedy/maglev0)

## Description

`maglev0` is a proof-of-concept tool to implement "maglev"-style consistent
hashing with Linux's CLUSTERIP iptables rules.

CLUSTERIP allows for N servers to share the burden of respondig to requests on
an IP (vip), contrast to the traditional HA Active/Standby failover techniques
where N=2. ("[seesaw](https://github.com/google/seesaw)"-style)

The hard part is managing which nodes respond to which requests. With
Active/Standby techniques you can use off-the-shelf clustering software
(keepalived, seesaw, heartbeat, etc). When N>2, something has to divide up the
work fairly and handle re-balancing when nodes change. This is what `maglev0`
does.

`maglev0` uses [zookeeper](https://zookeeper.apache.org/) for sharing a
consistent view of the world with peers. For balancing and reacting to node
changes, it uses Google's
[maglev](http://static.googleusercontent.com/media/research.google.com/en//pubs/archive/44824.pdf)
hashing algorithm to minimize disruption.

![maglev hashing](https://github.com/solarkennedy/maglev0/raw/master/maglev0.png)

## Limitations

`maglev0` doesn't actually load balance anything, really. It is more like a
"router-less router", where each node opts in to responding for packets it
thinks it is responsible for, ignoring the rest. It is more akin to
[ECMP](https://en.wikipedia.org/wiki/Equal-cost_multi-path_routing) without the
router help. This is the reasoning behind the `0` in `maglev0`: It implements
the first "layer" of routing that maglev-powered loadbalancers need, the
ECMP-style routing.

Future work might include using something other than CLUSTERIP, which is an old
(underrated?) technology that does not work in "cloud" environments.
Additionally, the design of CLUSTERIP requires that all *inbound* traffic reach
all nodes. For some workloads this is unacceptable.

Also I'm pretty sure I'm not using the algorithm correctly, it doesn't "seem"
very balanced (could be due to the small ring size)

## Setup

If your gopath is ready, you can use `go install`:

    go install github.com/solarkennedy/maglev0

Before `maglev0` can manipulate iptables for CLUSTERIP, the rule must be added.
This usually would happen with a startup script:

    iptables --insert INPUT --destination 198.51.100.1 \
      --jump CLUSTERIP --new --hashmode sourceip --clustermac 01:aa:7b:47:f7:d7 \
      --total-nodes 5 --local-node 1 --interface eth0

Then you can run `maglev0` to manage which nodes that CLUSTERIP responds to:

    maglev0 --my-id=1 --total-nodes=5 --cluster-ip=198.51.100.1

# Usage
```
Usage:
  maglev0 [--my-id=ID] [--total-nodes=N] [--zk=ZK] [--cluster-ip=IP]

Options:
  --my-id=<ID>      Specify the local node id, starting from 1. Must be unique throughout the cluster. [default: 1]
  --total-nodes=<N> Total nodes in the cluster. Can be more than the number of physical nodes available. [default: 5]
  --zk=<ZK>         Zookeeper connection string in csv format. [default: localhost:2181]
  --cluster-ip=<IP> Cluster ip (vip) to manage. [default: 198.51.100.1]
  -h, --help        Show this screen
```

## Example Output

```
sudo ./maglev0
map[--my-id:1 --total-nodes:<nil> --zk:localhost:2181 --cluster-ip:<nil>]
2016/11/20 17:54:02 Connected to [::1]:2181
2016/11/20 17:54:02 Authenticated: id=96972190335828052, timeout=4000
2016/11/20 17:54:02 Re-submitting `0` credentials after reconnect
...

[1 3 4]
backend-4 is responsible for node 1
backend-3 is responsible for node 2
backend-1 (me!) is responsible for node 3
backend-4 is responsible for node 4
backend-4 is responsible for node 5
CLUSTERIP Nodes:  3

....

[1 3]
backend-1 (me!) is responsible for node 1
backend-3 is responsible for node 2
backend-1 (me!) is responsible for node 3
backend-3 is responsible for node 4
backend-1 (me!) is responsible for node 5
CLUSTERIP Nodes:  1,3,5
```


## Warning

Don't use this. It is just a proof-of-concept.

## License

Apache2
