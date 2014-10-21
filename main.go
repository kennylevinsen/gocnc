package main

import "github.com/joushou/gocnc/gcode"

import "github.com/joushou/gocnc/optimizer"

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
	doc = optimizer.FillRemover(doc)
	doc = optimizer.FeedratePatcher(doc)
	doc = optimizer.CodeSaver(doc)
	doc = optimizer.LinearMoveSaver(doc)
	s := doc.Export(-1)
	fmt.Printf("%s\n", s)

}
