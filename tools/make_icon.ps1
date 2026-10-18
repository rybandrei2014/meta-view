# Regenerates icon.ico (classic BMP-encoded multi-size ICO) from assets/icon.png,
# then rsrc converts it to the .syso resources that Go links into Windows binaries.
#   go install github.com/akavel/rsrc@latest
#   rsrc -ico icon.ico -o rsrc_windows_amd64.syso
#   rsrc -ico icon.ico -arch arm64 -o rsrc_windows_arm64.syso
Add-Type -AssemblyName System.Drawing
$srcPath = Join-Path $PSScriptRoot "..\assets\icon.png"
$outPath = Join-Path $PSScriptRoot "..\icon.ico"
$sizes = 16, 24, 32, 48, 64, 128

$src = [System.Drawing.Image]::FromFile($srcPath)
$images = @()
foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap $s, $s
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = "HighQualityBicubic"
    $g.DrawImage($src, 0, 0, $s, $s)
    $g.Dispose()
    $images += ,$bmp
}
$src.Dispose()

$out = New-Object System.IO.MemoryStream
$bw = New-Object System.IO.BinaryWriter($out)
$bw.Write([uint16]0); $bw.Write([uint16]1); $bw.Write([uint16]$images.Count)
$offset = 6 + 16 * $images.Count
$dib = @()
foreach ($bmp in $images) {
    $s = $bmp.Width
    $ms = New-Object System.IO.MemoryStream
    $dw = New-Object System.IO.BinaryWriter($ms)
    # BITMAPINFOHEADER (height doubled: XOR image + AND mask)
    $maskRow = [math]::Ceiling($s / 32) * 4
    $xorLen = $s * $s * 4
    $dw.Write([uint32]40); $dw.Write([int32]$s); $dw.Write([int32](2 * $s))
    $dw.Write([uint16]1); $dw.Write([uint16]32); $dw.Write([uint32]0)
    $dw.Write([uint32]($xorLen + $maskRow * $s))
    $dw.Write([int32]0); $dw.Write([int32]0); $dw.Write([uint32]0); $dw.Write([uint32]0)
    # BGRA pixels, bottom-up
    for ($y = $s - 1; $y -ge 0; $y--) {
        for ($x = 0; $x -lt $s; $x++) {
            $c = $bmp.GetPixel($x, $y)
            $dw.Write([byte]$c.B); $dw.Write([byte]$c.G); $dw.Write([byte]$c.R); $dw.Write([byte]$c.A)
        }
    }
    # AND mask: all zero (alpha carries transparency)
    $zero = New-Object byte[] $maskRow
    for ($y = 0; $y -lt $s; $y++) { $dw.Write($zero) }
    $dw.Flush()
    $dib += ,$ms.ToArray()
    $ms.Dispose()
}
foreach ($b in $dib) {
    $bw.Write([byte]0); $bw.Write([byte]0); $bw.Write([byte]0); $bw.Write([byte]0)
    $bw.Write([uint16]1); $bw.Write([uint16]32)
    $bw.Write([uint32]$b.Length); $bw.Write([uint32]$offset)
    $offset += $b.Length
}
for ($i = 0; $i -lt $sizes.Count; $i++) {
    $bw.Seek(6 + 16 * $i, [System.IO.SeekOrigin]::Begin) | Out-Null
    $bw.Write([byte]$sizes[$i]); $bw.Write([byte]$sizes[$i])
}
$bw.Seek(0, [System.IO.SeekOrigin]::End) | Out-Null
foreach ($b in $dib) { $bw.Write($b) }
$bw.Flush()
[System.IO.File]::WriteAllBytes($outPath, $out.ToArray())
$bw.Dispose()
Write-Host ("wrote {0} ({1:n0} bytes)" -f $outPath, (Get-Item $outPath).Length)
