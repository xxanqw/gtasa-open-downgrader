package mtainstall

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/sassoftware/relic/v8/lib/authenticode"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

// VerifiedInstaller identifies the exact file authenticated during download.
type VerifiedInstaller struct {
	path   string
	sha256 [sha256.Size]byte
}

func (installer VerifiedInstaller) Path() string { return installer.path }

// Validate checks that the cached file is unchanged immediately before it runs.
func (installer VerifiedInstaller) Validate(ctx context.Context) error {
	info, err := os.Lstat(installer.path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxInstaller {
		return fmt.Errorf("MTA installer must be an ordinary file no larger than 200 MiB")
	}
	file, err := os.Open(installer.path)
	if err != nil {
		return err
	}
	defer file.Close()
	checksum := sha256.New()
	if _, err := fsutil.Copy(ctx, checksum, io.LimitReader(file, maxInstaller+1)); err != nil {
		return err
	}
	if [sha256.Size]byte(checksum.Sum(nil)) != installer.sha256 {
		return fmt.Errorf("cached MTA installer checksum mismatch, download it again")
	}
	return ctx.Err()
}

func verifySignature(file *os.File, size int64) (string, error) {
	image, err := pe.NewFile(file)
	if err != nil {
		return "", err
	}
	var table pe.DataDirectory
	const security = pe.IMAGE_DIRECTORY_ENTRY_SECURITY
	switch header := image.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		table = header.DataDirectory[security]
	case *pe.OptionalHeader64:
		table = header.DataDirectory[security]
	}
	// The security directory uses a file offset. Bound the certificate table
	// before signature parsing and require it to end at EOF.
	if table.VirtualAddress == 0 || table.Size < 8 || table.Size > 1<<20 ||
		int64(table.VirtualAddress)+int64(table.Size) != size {
		return "", fmt.Errorf("missing or invalid Authenticode certificate table")
	}
	signatures, err := authenticode.VerifyPE(file, false)
	if err != nil {
		return "", err
	}
	for _, signature := range signatures {
		subject := signature.Certificate.Subject
		if signature.ImageHashFunc != crypto.SHA256 ||
			subject.CommonName != "Open Source Developer Kevin Gross" ||
			len(subject.Organization) != 1 || subject.Organization[0] != "Open Source Developer" ||
			len(subject.Country) != 1 || subject.Country[0] != "AR" {
			continue
		}
		if err := signature.VerifyChain(nil, nil, x509.ExtKeyUsageCodeSigning); err != nil {
			return "", fmt.Errorf("MTA signing certificate chain: %w", err)
		}
		return hex.EncodeToString(signature.Indirect.MessageDigest.Digest), nil
	}
	return "", fmt.Errorf("missing SHA-256 signature from the MTA publisher")
}
