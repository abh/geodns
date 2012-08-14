package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
)

type User struct {
    Name string `json:"foo"`
}

// http://stackoverflow.com/questions/9801312/golang-nested-properties-for-structs-with-unknown-property-names
type Zone struct {
    Servers map[string]interface{}
}

func main() {

    var objmap map[string]json.RawMessage

    b, err := ioutil.ReadFile("ntppool.org.json")
    if err != nil {
        panic(err)
    }

    if err == nil {
        err := json.Unmarshal(b, &objmap)
        if err != nil {
            panic(err)
        }
        var str string
        err = json.Unmarshal(objmap["foo"], &str)
        fmt.Println(str)
    }

    user := &User{Name: "Frank"}
    c, err := json.Marshal(user)
    if err != nil {
        fmt.Println(err)
        return
    }
    fmt.Println(string(c))
}
