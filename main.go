package main

import (
	"fmt"
	"github.com/kkdai/maglev"
	"github.com/samuel/go-zookeeper/zk"
	"log"
	"time"
)

type State struct {
	maglev     *maglev.Maglev
	my_id      int
	ring_size  int
	table_size uint64
	zk_chroot  string
	zk_conn    *zk.Conn
}

func (s *State) PrintZk() {
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

func (s *State) RegisterZk() {
	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	_, err := s.zk_conn.Create(s.zk_chroot, []byte("data-parent"), flags, acl)

	node := fmt.Sprintf("%s/%d", s.zk_chroot, s.my_id)
	fmt.Println("Registering under %s", node)
	flags = int32(zk.FlagEphemeral)
	acl = zk.WorldACL(zk.PermAll)
	_, err = s.zk_conn.Create(node, []byte("here"), flags, acl)
	if err != nil {
		panic(err)
	}

}

func main() {
	zk_conn, _, zk_err := zk.Connect([]string{"10.0.2.10"}, time.Second)
	var names []string
	for i := 0; i < 5; i++ {
		names = append(names, fmt.Sprintf("backend-%d", i))
	}
	state := State{
		my_id:      1,
		ring_size:  5,  //
		table_size: 13, // "M" in the paper, must be prime
		zk_chroot:  "/maglev0",
		zk_conn:    zk_conn,
		maglev:     maglev.NewMaglev(names, 13),
	}
	time.Sleep(1 * time.Second)

	if zk_err != nil {
		panic(zk_err)
	}
	state.RegisterZk()
	state.PrintZk()

	v, err := state.maglev.Get("IP1")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("node1:", v)
	v, _ = state.maglev.Get("IP2")
	log.Println("node2:", v)
	v, _ = state.maglev.Get("IPasdasdwni2")
	log.Println("node3:", v)

	if err := state.maglev.Remove("backend-0"); err != nil {
		log.Fatal("Remove failed", err)
	}
	v, _ = state.maglev.Get("IPasdasdwni2")
	log.Println("node3-D:", v)
	//node3-D: Change from "backend-0" to "backend-1"
}
