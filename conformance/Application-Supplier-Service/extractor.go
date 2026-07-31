package main

import (
    "archive/tar"
    "archive/zip"
    "compress/gzip"
    "io"
    "os"
    "path/filepath"
)

func ExtractZip(src, dest string) error {

    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer r.Close()

    for _, f := range r.File {

        path := filepath.Join(dest, f.Name)

        if f.FileInfo().IsDir() {
            os.MkdirAll(path, 0755)
            continue
        }

        os.MkdirAll(filepath.Dir(path), 0755)

        rc, _ := f.Open()

        out, _ := os.Create(path)

        io.Copy(out, rc)

        out.Close()
        rc.Close()
    }

    return nil
}

func ExtractTarGz(src, dest string) error {

    file, err := os.Open(src)
    if err != nil {
        return err
    }

    gzr, err := gzip.NewReader(file)
    if err != nil {
        return err
    }

    tr := tar.NewReader(gzr)

    for {
        header, err := tr.Next()

        if err == io.EOF {
            break
        }

        if err != nil {
            return err
        }

        target := filepath.Join(dest, header.Name)

        switch header.Typeflag {

        case tar.TypeDir:
            os.MkdirAll(target, 0755)

        case tar.TypeReg:
            os.MkdirAll(filepath.Dir(target), 0755)

            f, _ := os.Create(target)

            io.Copy(f, tr)

            f.Close()
        }
    }

    return nil
}
