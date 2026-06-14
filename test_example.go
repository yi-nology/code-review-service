package main

import "fmt"

// TestFunction demonstrates a potential issue
func TestFunction() {
    var data map[string]string
    data["key"] = "value" // potential nil map panic
    fmt.Println(data)
}
