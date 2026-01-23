package wfm

import "github.com/margo/sandbox/shared-lib/crypto"

func WithRequestSigner(opt crypto.HTTPSigner) {}

func WithTls() {}

func WithNoTls() {}
