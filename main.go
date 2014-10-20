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
	doc = optimizer.FilemarkRemover(doc)
	doc = optimizer.FeedratePatcher(doc)
	fmt.Printf("%s\n", doc.ToString())

}
