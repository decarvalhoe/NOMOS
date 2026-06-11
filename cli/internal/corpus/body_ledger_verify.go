package corpus

import "fmt"

// VerifyCorpusBodyLedgerProofs verifies every Merkle inclusion proof in the
// ledger against its recorded root (VRC-07 #553). Each leaf hash is RECOMPUTED
// from the ledger row itself before the proof path is walked: trusting the
// stored leaf_hash would be circular, and a tampered row shipped with its
// original proof must fail. The walk mirrors attachBodyLedgerMerkle exactly —
// one leaf per source row plus one leaf per root segment (ParentSegmentID
// empty); child segments carry no leaves. The verified leaf count must match
// MerkleSummary.LeafCount so injected or dropped leaves are also caught.
//
// Ledgers without a Merkle summary (pre-CKM-06) are rejected as unverifiable
// rather than silently passed.
func VerifyCorpusBodyLedgerProofs(ledger CorpusBodyLedger) error {
	if ledger.Merkle == nil || ledger.Merkle.Root == "" {
		return fmt.Errorf("body ledger carries no merkle summary; nothing to verify")
	}
	root := ledger.Merkle.Root
	verified := 0
	for i := range ledger.Sources {
		source := ledger.Sources[i]
		if source.MerkleProof == nil {
			return fmt.Errorf("source %s carries no merkle proof", source.SourceID)
		}
		leaf := bodyLedgerSourceLeafHash(source)
		if leaf != source.MerkleProof.LeafHash {
			return fmt.Errorf(
				"source %s leaf hash mismatch: ledger row was altered after proof generation",
				source.SourceID,
			)
		}
		if err := VerifyMerkleProof(leaf, *source.MerkleProof, root); err != nil {
			return fmt.Errorf("source %s: %w", source.SourceID, err)
		}
		verified++
		for j := range source.Segments {
			segment := source.Segments[j]
			if segment.ParentSegmentID != "" {
				continue
			}
			if segment.MerkleProof == nil {
				return fmt.Errorf("segment %s carries no merkle proof", segment.SegmentID)
			}
			leaf := bodyLedgerSegmentLeafHash(segment)
			if leaf != segment.MerkleProof.LeafHash {
				return fmt.Errorf(
					"segment %s leaf hash mismatch: ledger row was altered after proof generation",
					segment.SegmentID,
				)
			}
			if err := VerifyMerkleProof(leaf, *segment.MerkleProof, root); err != nil {
				return fmt.Errorf("segment %s: %w", segment.SegmentID, err)
			}
			verified++
		}
	}
	if verified != ledger.Merkle.LeafCount {
		return fmt.Errorf(
			"verified %d inclusion proofs but the ledger declares %d leaves",
			verified, ledger.Merkle.LeafCount,
		)
	}
	return nil
}
