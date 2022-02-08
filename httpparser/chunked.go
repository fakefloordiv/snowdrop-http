package httpparser

type OnBodyCallback func([]byte)

type chunkedBodyParser struct {
	callback       OnBodyCallback
	state          chunkedBodyState
	chunkLength    int
	chunkBodyBegin int

	maxChunkSize int
}

func NewChunkedBodyParser(callback OnBodyCallback, maxChunkSize int) *chunkedBodyParser {
	return &chunkedBodyParser{
		callback: callback,
		state:    chunkLength,
		// as chunked requests aren't obligatory, we better keep the buffer unallocated until
		// we'll need it
		maxChunkSize: maxChunkSize,
	}
}

func (p *chunkedBodyParser) Clear() {
	p.state = chunkLength
	p.chunkLength = 0
}

func (p *chunkedBodyParser) Feed(data []byte) (done bool, extraBytes []byte, err error) {
	if p.state == transferCompleted {
		/*
			It returns extra-bytes as parser must know, that it's his job now

			But if parser is feeding again, it means only that we really need
			to parse one more chunked body
		*/
		p.Clear()
	}
	if len(data) == 0 {
		return false, nil, nil
	}

	for i, char := range data {
		switch p.state {
		case chunkLength:
			switch char {
			case '\r':
				p.state = chunkLengthCR
			case '\n':
				if p.chunkLength == 0 {
					p.state = lastChunk
					break
				}

				p.chunkBodyBegin = i + 1
				p.state = chunkBody
			default:
				// TODO: add support of trailers
				// TODO: replace with ascii.IsPrint()
				if (char < '0' && char > '9') && (char < 'a' && char > 'f') && (char < 'A' && char > 'F') {
					// non-printable ascii-character
					p.complete()

					return true, nil, ErrInvalidChunkSize
				}

				p.chunkLength = (p.chunkLength << 4) + int((char&0xF)+9*(char>>6))

				if p.chunkLength > p.maxChunkSize {
					p.complete()

					return true, nil, ErrTooBigChunkSize
				}
			}
		case chunkLengthCR:
			if char != '\n' {
				p.complete()

				return true, nil, ErrInvalidChunkSplitter
			}

			if p.chunkLength == 0 {
				p.state = lastChunk
				break
			}

			p.chunkBodyBegin = i + 1
			p.state = chunkBody
		case chunkBody:
			p.chunkLength--

			if p.chunkLength == 0 {
				p.state = chunkBodyEnd
			}
		case chunkBodyEnd:
			p.callback(data[p.chunkBodyBegin:i])

			switch char {
			case '\r':
				p.state = chunkBodyCR
			case '\n':
				p.state = chunkLength
			default:
				p.complete()

				return true, nil, ErrInvalidChunkSplitter
			}
		case chunkBodyCR:
			if char != '\n' {
				p.complete()

				return true, nil, ErrInvalidChunkSplitter
			}

			p.state = chunkLength
		case lastChunk:
			switch char {
			case '\r':
				p.state = lastChunkCR
			case '\n':
				p.complete()

				return true, data[i+1:], nil
			default:
				// looks sad, received everything, and fucked up in the end
				// or this was made for special? Oh god
				p.complete()

				return true, nil, ErrInvalidChunkSplitter
			}
		case lastChunkCR:
			if char != '\n' {
				p.complete()

				return true, nil, ErrInvalidChunkSplitter
			}

			p.complete()

			return true, data[i+1:], nil
		}
	}

	if p.state == chunkBody {
		p.callback(data[p.chunkBodyBegin:])
	}

	p.chunkBodyBegin = 0

	return false, nil, nil
}

func (p *chunkedBodyParser) complete() {
	p.state = transferCompleted
}
