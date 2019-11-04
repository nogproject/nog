import { expect } from 'chai'

{
  asXiBUnit
} = NogFmt

{
  round
} = Math


describe 'NogFmt', ->
  describe 'asXiBUnit()', ->
    K = 1024
    M = 1024 * K
    G = 1024 * M
    T = 1024 * G
    P = 1024 * T

    expectUnit = (v, e) ->
      [value, unit] = e.split(' ')
      expect(v).to.eql {value, unit}
      expect(typeof(v.value)).to.eql 'string'

    it 'handles 0', ->
      expectUnit(asXiBUnit(0), '0 bytes')

    it 'handles singular', ->
      expectUnit(asXiBUnit(1), '1 byte')

    it 'formats bytes as integers', ->
      expectUnit(asXiBUnit(2), '2 bytes')
      expectUnit(asXiBUnit(1023), '1023 bytes')

    it 'formats < 10 to fixed 1 (KiB)', ->
      expectUnit(asXiBUnit(K), '1.0 KiB')
      expectUnit(asXiBUnit(round(9.94 * K)), '9.9 KiB')

    it 'formats >= 10 to integers (KiB)', ->
      expectUnit(asXiBUnit(round(9.96 * K)), '10 KiB')
      expectUnit(asXiBUnit(M - 1), '1024 KiB')

    it 'formats < 10 to fixed 1 (MiB)', ->
      expectUnit(asXiBUnit(M), '1.0 MiB')
      expectUnit(asXiBUnit(round(9.94 * M)), '9.9 MiB')

    it 'formats >= 10 to integers (MiB)', ->
      expectUnit(asXiBUnit(round(9.96 * M)), '10 MiB')
      expectUnit(asXiBUnit(G - 1), '1024 MiB')

    it 'formats < 10 to fixed 1 (TiB)', ->
      expectUnit(asXiBUnit(T), '1.0 TiB')
      expectUnit(asXiBUnit(round(9.94 * T)), '9.9 TiB')

    it 'formats >= 10 to integers (TiB)', ->
      expectUnit(asXiBUnit(round(9.96 * T)), '10 TiB')
      expectUnit(asXiBUnit(P - 1), '1024 TiB')

    it 'formats PiB as TiB', ->
      expect(asXiBUnit(P), '1024 TiB')
      expect(asXiBUnit(10 * P), '10240 TiB')
      expect(asXiBUnit(100 * P), '102400 TiB')
