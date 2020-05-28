# c14n

 [![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/mod/github.com/ucarion/c14n?tab=overview)

This package is a Golang implementation of XML Canonicalization ("c14n"). In
particular, it implements the [Exclusive Canonical XML][w3] specification, which
is the recommended canonicalization scheme used in SAML.

If you're looking to canonicalize XML because you're implementing SAML or XML
Digital Signature, consider using [`github.com/ucarion/saml`][saml] or
[`github.com/ucarion/dsig`][dsig], which are implemented using this package.

[w3]: https://www.w3.org/TR/xml-exc-c14n/
[saml]: https://github.com/ucarion/saml
[dsig]: https://github.com/ucarion/dsig

## Installation

Install this package by running:

```bash
go get github.com/ucarion/c14n
```

## Usage

The most common way to use this package is to call `c14n.Canonicalize` with a
`xml.Decoder`:

```go
input := `<foo z="2" a="1"><bar /></foo>`
decoder := xml.NewDecoder(strings.NewReader(input))
out, err := c14n.Canonicalize(decoder)
fmt.Println(string(out), err)
// Output:
// <foo a="1" z="2"><bar></bar></foo> <nil>
```
