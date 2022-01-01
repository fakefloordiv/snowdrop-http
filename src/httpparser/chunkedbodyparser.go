package httpparser

import (
	"strconv"
)


const (
	_maxChunkSize = 65535
	_maxHexChunkSize = "FFFF"
	_chunkSizeBitSize = 16
)

type (
	ChunkSizeType uint16
	OnBodyCallback func([]byte)


)

type chunkedBodyParser struct {
	callback 					OnBodyCallback
	state 						ParsingState
	currentChunkLength  		ChunkSizeType
	tempBuf						[]byte
	chunksReceived				int
}

func NewChunkedBodyParser(callback OnBodyCallback) *chunkedBodyParser {
	return &chunkedBodyParser{
		callback:                  callback,
		state:                     ExpectingChunkLength,
		currentChunkLength:        0,
		tempBuf:                   make([]byte, 0, _maxChunkSize),
		chunksReceived:            0,
	}
}

func (p *chunkedBodyParser) Reuse(callback OnBodyCallback) {
	p.callback = callback
	p.state = ExpectingChunkLength
	p.currentChunkLength = 0
	p.tempBuf = nil
	p.chunksReceived = 0
}

func (p *chunkedBodyParser) Feed(data []byte) (done bool, chunkErr error) {
	if p.state == BodyCompleted {
		return true, nil
	}
	if len(data) == 0 {
		return false, nil
	}

	for _, char := range data {
		if char == '\r' {
			continue
		}

		if char == '\n' {
			if p.state == ExpectingChunkLength {
				chunkLength, err := strconv.ParseUint(string(p.tempBuf), 16, _chunkSizeBitSize)

				if err != nil {
					return true, InvalidChunkLength
				}

				p.currentChunkLength = ChunkSizeType(chunkLength)
				p.tempBuf = nil
				p.state = ExpectingChunk
			} else {
				if p.currentChunkLength == 0 {
					p.state = BodyCompleted
					return true, nil
				}
				if ChunkSizeType(len(p.tempBuf)) < p.currentChunkLength {
					return true, NotEnoughChunk
				}

				p.callback(p.tempBuf)
				p.tempBuf = nil
				p.chunksReceived++
				p.state = ExpectingChunkLength
			}
		} else {
			p.tempBuf = append(p.tempBuf, char)

			switch {
			case p.state == ExpectingChunkLength && len(p.tempBuf) > len(_maxHexChunkSize):
				return true, TooBigChunkSize
			case p.state == ExpectingChunk && len(p.tempBuf) >= _maxChunkSize:
				return true, TooBigChunk
			}
		}
	}

	return false, nil
}

func quote(data []byte) string {
	/*
	Isn't removed yet as sometimes I need this for debug

	Don't ask me why I'm not using debugger. Once I used,
	an it fucked up my videodriver. I don't wanna try again
	 */
	return strconv.Quote(string(data))
}