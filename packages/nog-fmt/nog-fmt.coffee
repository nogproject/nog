NogFmt = {}

# The file size will computed in kibibytes and formatted to two different
# cases: (1) single leading numbers with one decimal place and (2) multiple
# leading numbers without any decimal places.  During differentiating the two
# cases, we check against the rounded result of case (1) to avoid inconsistent
# formatting such as:
#
# - 10239 bytes = 9.999 KiB -> toFixed(1): 10.0 KiB
# - 10240 bytes = 10.00 KiB -> toFixed(0): 10 KiB

asXiBUnit = (size) ->
  if size == 0
    return {value: '0', unit: 'bytes'}
  if size == 1
    return {value: '1', unit: 'byte'}
  e = Math.floor(Math.log(size) / Math.log(1024))
  e = Math.min(e, 4)
  human = (size / Math.pow(1024, e))
  if e > 0 && human.toFixed(1) < 10
    human = human.toFixed(1)
  else
    human = human.toFixed(0)
  unit = ['bytes', 'KiB', 'MiB', 'GiB', 'TiB'][e]
  return {
    value: human
    unit
  }


NogFmt.asXiBUnit = asXiBUnit
