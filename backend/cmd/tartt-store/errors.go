package main

func mustMsg(err error, msg string) {
	if err == nil {
		return
	}
	lg.Fatalw(msg, "err", err)
}

func mustGpg(err error)          { mustMsg(err, "Failed to run gpg2.") }
func mustGunzip(err error)       { mustMsg(err, "Failed to gunzip.") }
func mustGzip(err error)         { mustMsg(err, "Failed to gzip.") }
func mustLoad(err error)         { mustMsg(err, "Failed to load data.") }
func mustLoadManifest(err error) { mustMsg(err, "Failed to load manifest.") }
func mustLoadSecret(err error)   { mustMsg(err, "Failed to load secret.") }
func mustManifest(err error)     { mustMsg(err, "Failed to write manifest.") }
func mustNewSecret(err error)    { mustMsg(err, "Failed to generate secret.") }
func mustReceive(err error)      { mustMsg(err, "Failed to receive data.") }
func mustSave(err error)         { mustMsg(err, "Failed to save data.") }
func mustSecret(err error)       { mustMsg(err, "Failed to pass secret to gpg.") }
func mustSend(err error)         { mustMsg(err, "Failed to send data.") }
func mustTar(err error)          { mustMsg(err, "Failed to tar chunk.") }
func mustUntar(err error)        { mustMsg(err, "Failed to untar chunk.") }
func mustUnzstd(err error)       { mustMsg(err, "Failed to unzstd.") }
func mustZstd(err error)         { mustMsg(err, "Failed to zstd.") }
