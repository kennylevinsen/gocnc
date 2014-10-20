package main

import "github.com/joushou/gogcode"
import "io/ioutil"
import "fmt"

func main() {
	b, err := ioutil.ReadFile("/Users/kenny/Documents/holddown.nc")
	if err == nil {
		fmt.Printf("No such file: %s\n", err)
	}
	test := string(b)
	doc := gogcode.Parse(test)
	fmt.Printf("%s\n", doc.ToString())
}
