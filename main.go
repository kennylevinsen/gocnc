package main

import "github.com/joushou/gocnc/gcode"

import "github.com/joushou/gocnc/vm"

import "io/ioutil"
import "flag"
import "fmt"

func main() {
	flag.Parse()
	b, err := ioutil.ReadFile(flag.Args()[0])
	if err != nil {
		fmt.Printf("No such file: %s\n", err)
	}
	test := string(b)
	doc := gcode.Parse(test)
	var m vm.Machine
	m.Init()
	m.Process(doc)
	m.OptimizeMoves()
	m.OptimizeLifts()
	fmt.Printf(m.Export(4))
	//s := doc.Export(-1)
	//fmt.Printf("%s\n", s)

}
