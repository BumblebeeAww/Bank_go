package main

import (
	"bytes"    
    "io"
    "os"
    "strings"
	
    "golang.org/x/crypto/openpgp"
)

// LoadPublicKey загружает публичный PGP-ключ из файла
func LoadPublicKey(path string) (openpgp.EntityList, error) {
    keyFile, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer keyFile.Close()

    return openpgp.ReadArmoredKeyRing(keyFile)
}

// LoadPrivateKey загружает приватный PGP-ключ из файла
func LoadPrivateKey(path string) (openpgp.EntityList, error) {
    keyFile, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer keyFile.Close()

    return openpgp.ReadArmoredKeyRing(keyFile)
}

// EncryptWithPGP шифрует текст публичным ключом PGP
func EncryptWithPGP(plaintext string, pubKey openpgp.EntityList) (string, error) {
    var buf bytes.Buffer
    w, err := openpgp.Encrypt(&buf, pubKey, nil, nil, nil)
    if err != nil {
        return "", err
    }
    _, err = w.Write([]byte(plaintext))
    if err != nil {
        return "", err
    }
    err = w.Close()
    if err != nil {
        return "", err
    }
    return buf.String(), nil
}

// DecryptWithPGP расшифровывает PGP-текст приватным ключом
func DecryptWithPGP(ciphertext string, privKey openpgp.EntityList) (string, error) {
    r := strings.NewReader(ciphertext)
    md, err := openpgp.ReadMessage(r, privKey, nil, nil)
    if err != nil {
        return "", err
    }
    decryptedBytes, err := io.ReadAll(md.UnverifiedBody)
    if err != nil {
        return "", err
    }
    return string(decryptedBytes), nil
}