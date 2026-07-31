package main

import (
    "fmt"
    "strings"
)

func ValidateHelmComponent(
    c Component,
) error {

    repo, ok :=
        c.Properties["repository"]

    if !ok {

        return fmt.Errorf(
            "repository missing",
        )
    }

    repository :=
        repo.(string)

    if strings.HasPrefix(
        repository,
        "https://",
    ) || strings.HasPrefix(
        repository,
        "http://",
    ) {

        return CheckHTTPS(repository)
    }

    if strings.HasPrefix(
        repository,
        "oci://",
    ) {

        return CheckOCI(repository)
    }

    return fmt.Errorf(
        "unsupported repository: %s",
        repository,
    )
}