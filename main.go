package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/kkdai/maglev"
	"github.com/samuel/go-zookeeper/zk"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

type State struct {
	maglev     *maglev.Maglev
	my_id      int
	ring_size  int
	zk_chroot  string
	cluster_ip string
	zk_conn    *zk.Conn
}

func (s *State) PrintZK() {
	fmt.Println("Printing what is in zk:")
	children, stat, ch, err := s.zk_conn.ChildrenW(s.zk_chroot)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v %+v\n", children, stat)
	e := <-ch
	fmt.Printf("%+v\n", e)
	fmt.Println("Done.")
}

func (s *State) RegisterZK() {
	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	_, err := s.zk_conn.Create(s.zk_chroot, []byte(""), flags, acl)

	node := fmt.Sprintf("%s/%d", s.zk_chroot, s.my_id)
	fmt.Println("Registering under ", node)
	flags = int32(zk.FlagEphemeral)
	acl = zk.WorldACL(zk.PermAll)
	_, err = s.zk_conn.Create(node, []byte(""), flags, acl)
	if err != nil {
		panic(err)
	}

}

func (s *State) MirrorZK() (chan []string, chan error) {
	snapshots := make(chan []string)
	errors := make(chan error)
	go func() {
		for {
			snapshot, _, events, err := s.zk_conn.ChildrenW(s.zk_chroot)
			if err != nil {
				errors <- err
				return
			}
			snapshots <- snapshot
			evt := <-events
			if evt.Err != nil {
				errors <- evt.Err
				return
			}
		}
	}()
	return snapshots, errors
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (s *State) SyncBackends() {
	nodes, _, _ := s.zk_conn.Children(s.zk_chroot)
	fmt.Println(nodes)
	for node := 1; node <= s.ring_size; node++ {
		node_str := fmt.Sprintf("%d", node)
		if stringInSlice(node_str, nodes) {
			s.maglev.Add(fmt.Sprintf("backend-%d", node))
		} else {
			s.maglev.Remove(fmt.Sprintf("backend-%d", node))
		}
	}
	s.FlushState()
}

func (s *State) RemoveNode(node int) {
	d := []byte(fmt.Sprintf("-%d\n", node))
	ioutil.WriteFile(s.GetClusterIPFile(), d, 0644)
}

func (s *State) AddNode(node int) {
	d := []byte(fmt.Sprintf("+%d\n", node))
	ioutil.WriteFile(s.GetClusterIPFile(), d, 0644)
}

func (s *State) GetClusterIPFile() string {
	return fmt.Sprintf("/proc/net/ipt_CLUSTERIP/%s", s.cluster_ip)
}

func (s *State) PrintState() {
	b, err := ioutil.ReadFile(s.GetClusterIPFile())
	if err != nil {
		panic(err)
	}
	fmt.Println("CLUSTERIP Nodes: ", string(b))
}

func (s *State) FlushState() {
	local_backend := fmt.Sprintf("backend-%d", s.my_id)
	for node := 1; node <= s.ring_size; node++ {
		backend, err := s.maglev.Get(fmt.Sprintf("%d", node))
		if err != nil {
			panic(err)
		}
		if local_backend == backend {
			fmt.Printf("%s (me!) is responsible for node %d\n", backend, node)
			s.AddNode(node)
		} else {
			fmt.Printf("%s is responsible for node %d\n", backend, node)
			s.RemoveNode(node)
		}
	}
	s.PrintState()
}

func (s *State) WatchForever() {
	snapshots, errors := s.MirrorZK()
	s.RegisterZK()
	s.SyncBackends()
	for {
		select {
		case <-snapshots:
			s.SyncBackends()
		case err := <-errors:
			panic(err)
		}
	}
}

func parseArgs() map[string]interface{} {
	usage := `maglev0 - Faux-maglev-style hashing for managing VIPs using Iptables CLUSTERIP and Zookeeper.

Usage:
  maglev0 [--my-id=ID] [--total-nodes=N] [--zk=ZK] [--cluster-ip=IP]

Options:
  --my-id=<ID>      Specify the local node id, starting from 1. Must be unique throughout the cluster. [default: 1]
  --total-nodes=<N> Total nodes in the cluster. Can be more than the number of physical nodes available. [default: 5]
  --zk=<ZK>         Zookeeper connection string in csv format. [default: localhost:2181]
  --cluster-ip=<IP> Cluster ip (vip) to manage. [default: 198.51.100.1]
  -h, --help        Show this screen
`
	arguments, err := docopt.Parse(usage, nil, true, "0.0.1", false)
	if err != nil {
		fmt.Println(arguments)
		fmt.Println(err)
		os.Exit(1)
	}
	return arguments
}

func main() {
	args := parseArgs()
	fmt.Println(args)
	var zk_string string
	if args["--zk"] == nil {
		zk_string = "localhost:2181"
	} else {
		zk_string = args["--zk"].(string)
	}
	zk_conn, _, zk_err := zk.Connect([]string{zk_string}, time.Second)
	if zk_err != nil {
		panic(zk_err)
	}

	var cluster_ip string
	if args["--cluster-ip"] == nil {
		cluster_ip = "198.51.100.1"
	} else {
		cluster_ip = args["--cluster-ip"].(string)
	}

	var my_id int
	if args["--my-id"] == nil {
		my_id = 1
	} else {
		my_id, _ = strconv.Atoi(args["--my-id"].(string))
	}

	var total_nodes int
	if args["--total-nodes"] == nil {
		total_nodes = 5
	} else {
		total_nodes, _ = strconv.Atoi(args["--total-nodes"].(string))
	}

	var names []string
	for i := 1; i <= total_nodes; i++ {
		names = append(names, fmt.Sprintf("backend-%d", i))
	}
	maglev_m := 13 // Must be prime per the paper

	state := State{
		my_id:      my_id,
		ring_size:  total_nodes, //
		cluster_ip: cluster_ip,
		zk_chroot:  "/maglev0",
		zk_conn:    zk_conn,
		maglev:     maglev.NewMaglev(names, uint64(maglev_m)),
	}

	state.WatchForever()
}
