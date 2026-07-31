package main

import (
    "fmt"
    "net/http"
    "os/exec"
    "strings"
)

func CheckHTTPS(url string) error {

    resp, err := http.Head(url)

    if err != nil {
        return err
    }

    if resp.StatusCode >= 400 {
        return fmt.Errorf(
            "url not reachable: %d",
            resp.StatusCode,
        )
    }

    return nil
}

func CheckOCI(location string) error {

    cmd := exec.Command(
        "oras",
        "manifest",
        "fetch",
        location,
    )

    out, err := cmd.CombinedOutput()

    if err != nil {

        return fmt.Errorf(
            "oci unreachable: %s",
            string(out),
        )
    }

    return nil
}

func CheckPackageLocation(
    location string,
) error {

    if strings.HasPrefix(
        location,
        "https://",
    ) {

        return CheckHTTPS(location)
    }

    if strings.HasPrefix(
        location,
        "http://",
    ) {

        return CheckHTTPS(location)
    }

    if strings.HasPrefix(
        location,
        "oci://",
    ) {

        return CheckOCI(location)
    }

    return fmt.Errorf(
        "unsupported packageLocation: %s",
        location,
    )
}