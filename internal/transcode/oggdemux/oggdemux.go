package oggdemux

import (
	"errors"
	"fmt"
	"io"
)

const errPrefix = "transcode/oggdemux: "
const maxAudioPacketSize = 64 * 1024

var oggPageStartMagicSignature = [4]byte{'O', 'g', 'g', 'S'}
var opusHeadMagicSignature = [8]byte{'O', 'p', 'u', 's', 'H', 'e', 'a', 'd'}
var opusTagsMagicSignature = [8]byte{'O', 'p', 'u', 's', 'T', 'a', 'g', 's'}

type opusPacketType int

const (
	opusPacketTypeIdHeader opusPacketType = iota
	opusPacketTypeCommentHeader
	opusPacketTypeAudioData
)

type OggOpusDemuxer struct {
	src io.Reader

	pageHeaderBuf          [27]byte
	pageSegTable           [255]byte
	pageSegIdx             int
	expectedNextPacketType opusPacketType
}

func NewOggOpusPacketReader(src io.Reader) *OggOpusDemuxer {
	return &OggOpusDemuxer{
		src:                    src,
		expectedNextPacketType: opusPacketTypeIdHeader,
	}
}

func (ood *OggOpusDemuxer) ReadAudioPacket(dst []byte) ([]byte, error) {
	for {
		dst, err := ood.readOpusPacket(dst)
		if err != nil {
			return nil, err
		}

		switch ood.expectedNextPacketType {
		case opusPacketTypeIdHeader:
			if len(dst) < len(opusHeadMagicSignature) {
				return nil, fmt.Errorf(errPrefix + "packet too small to contain opus head magic signature")
			}

			if sig := [len(opusHeadMagicSignature)]byte(dst[:len(opusHeadMagicSignature)]); sig != opusHeadMagicSignature {
				return nil, fmt.Errorf(errPrefix+"expected opus head magic signature, got %x", sig)
			}

			ood.expectedNextPacketType = opusPacketTypeCommentHeader
		case opusPacketTypeCommentHeader:
			if len(dst) < len(opusTagsMagicSignature) {
				return nil, fmt.Errorf(errPrefix + "packet too small to contain opus tags magic signature")
			}

			if sig := [len(opusTagsMagicSignature)]byte(dst[:len(opusTagsMagicSignature)]); sig != opusTagsMagicSignature {
				return nil, fmt.Errorf(errPrefix+"expected opus tags magic signature, got %x", sig)
			}

			ood.expectedNextPacketType = opusPacketTypeAudioData
		default:
			return dst, nil
		}
	}
}

func (ood *OggOpusDemuxer) readOpusPacket(dst []byte) ([]byte, error) {
	dst = dst[:0]

	for {
		if ood.pageSegIdx >= ood.pageNumSegments() {
			if err := ood.readPageHeader(); err != nil {
				return nil, err
			}

			// if not "continued packet" flag - reset dst
			if ood.pageHeaderBuf[5]&0x01 == 0 {
				dst = dst[:0]
			}
		}

		pageSegLen := int(ood.pageSegTable[ood.pageSegIdx])
		ood.pageSegIdx++

		if pageSegLen > 0 {
			startOffset := len(dst)

			if startOffset+pageSegLen > maxAudioPacketSize {
				return nil, fmt.Errorf(errPrefix+"audio packet size is larger than %d", maxAudioPacketSize)
			}

			dst = append(dst, make([]byte, pageSegLen)...)
			if _, err := io.ReadFull(ood.src, dst[startOffset:startOffset+pageSegLen]); err != nil {
				return nil, fmt.Errorf(errPrefix+"reading page segment data: %w", err)
			}
		}

		// 255 = continued in next page
		if pageSegLen != 255 {
			return dst, nil
		}
	}
}

func (ood *OggOpusDemuxer) readPageHeader() error {
	if _, err := io.ReadFull(ood.src, ood.pageHeaderBuf[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return io.EOF
		}
		return fmt.Errorf(errPrefix+"reading ogg page header: %w", err)
	}

	if sig := [len(oggPageStartMagicSignature)]byte(ood.pageHeaderBuf[:len(oggPageStartMagicSignature)]); sig != oggPageStartMagicSignature {
		return fmt.Errorf(errPrefix+"expected ogg page start magic signature, got: %x", sig)
	}

	if ood.pageHeaderBuf[4] != 0 {
		return fmt.Errorf(errPrefix+"unsupported ogg stream structure version: %d", ood.pageHeaderBuf[4])
	}

	ood.pageSegIdx = 0

	if ood.pageNumSegments() > 0 {
		if _, err := io.ReadFull(ood.src, ood.pageSegTable[:ood.pageNumSegments()]); err != nil {
			return fmt.Errorf(errPrefix+"reading ogg page segment table: %w", err)
		}
	}

	return nil
}

func (ood *OggOpusDemuxer) pageNumSegments() int {
	return int(ood.pageHeaderBuf[26])
}
