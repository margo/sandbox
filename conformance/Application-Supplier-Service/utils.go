package main


import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "gopkg.in/yaml.v3"
)


func FindApplicationDescription(root string) (string, error) {

    var found string

    err := filepath.Walk(
        root,
        func(path string, info os.FileInfo, err error) error {

            if err != nil || info == nil {
                return nil
            }

            if info.IsDir() {
                return nil
            }

            ext := strings.ToLower(
                filepath.Ext(path),
            )

            if ext != ".yaml" &&
                ext != ".yml" {

                return nil
            }

            data, err := os.ReadFile(path)

            if err != nil {
                return nil
            }

            var obj struct {
                Kind string `yaml:"kind"`
            }

            if err := yaml.Unmarshal(
                data,
                &obj,
            ); err != nil {

                return nil
            }

            if obj.Kind ==
                "ApplicationDescription" {

                found = path
            }

            return nil
        },
    )

    if err != nil {
        return "", err
    }

    if found == "" {

        return "",
            fmt.Errorf(
                "ApplicationDescription not found",
            )
    }

    return found, nil
}