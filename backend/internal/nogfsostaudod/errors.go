package nogfsostaudod

const ConfigErrorMessageTruncateLength = 500

func truncatedErrorMessage(s string) string {
	if len(s) <= ConfigErrorMessageTruncateLength {
		return s
	}
	return s[0:ConfigErrorMessageTruncateLength-3] + "..."
}
