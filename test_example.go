package main

import "fmt"

// TestFunction demonstrates a potential issue
func TestFunction() {
    data := make(map[string]string)
    data["key"] = "value"
    fmt.Println(data)
}
