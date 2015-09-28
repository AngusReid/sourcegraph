package local

import "sourcegraph.com/sourcegraph/go-vcs/vcs"

// blameFileByteRange calls r.BlameFile and filters the returned hunks
// to only those that contain bytes in [startByte, endByte). It is
// useful because r.BlameFile can only blame a whole file or a subset
// of lines (not a byte range), and it's often better to blame the
// whole file and post-filter so that vcsstore can cache entire files'
// blame outputs (instead of very many more byte ranges' blame
// outputs).
//
// The hunks returned by BlameFileByteRange are clipped so that their
// byte ranges do not extend outside of [startByte, endByte). However,
// their start and end lines are not clipped and reflect the original
// hunk's start and end lines.
func blameFileByteRange(r vcs.Blamer, path string, opt *vcs.BlameOptions, startByte, endByte int) ([]*vcs.Hunk, error) {
	hunks, err := r.BlameFile(path, opt)
	if err != nil {
		return nil, err
	}

	var hunks2 []*vcs.Hunk // filtered
	for _, h := range hunks {
		if h.StartByte <= endByte && h.EndByte > startByte {
			if h.StartByte < startByte {
				h.StartByte = startByte
			}
			if h.EndByte > endByte {
				h.EndByte = endByte
			}
			hunks2 = append(hunks2, h)
		}
	}
	return hunks2, nil
}
