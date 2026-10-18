import struct, os

os.makedirs("testfiles", exist_ok=True)

# WAV
rate, ch, bits = 44100, 2, 16
data = b"\x00\x00" * 4410  # 0.1s
fmt = struct.pack("<HHIIHH", 1, ch, rate, rate*ch*bits//8, ch*bits//8, bits)
wav = (b"RIFF" + struct.pack("<I", 36+len(fmt)+len(data)) + b"WAVE"
       + b"fmt " + struct.pack("<I", len(fmt)) + fmt
       + b"LIST" + struct.pack("<I", 8+12+16) + b"INFO"
       + b"INAM" + struct.pack("<I", 8) + b"Test Song\x00\x00"
       + b"IART" + struct.pack("<I", 8) + b"Tester\x00\x00"
       + b"data" + struct.pack("<I", len(data)) + data)
open("testfiles/test.wav","wb").write(wav)

# BMP 4x2 24bpp
px = bytes([0,0,255])*4 + b"\x00\x00\x00"
row = px
pixels = row*2
hdr = struct.pack("<IiiHHIIiiII", 40, 4, 2, 1, 24, 0, len(pixels), 2835, 2835, 0, 0)
bfh = b"BM" + struct.pack("<IHHI", 14+len(hdr)+len(pixels), 0, 0, 14+len(hdr))
open("testfiles/test.bmp","wb").write(bfh+hdr+pixels)

# GIF 2x2
gif = (b"GIF89a" + struct.pack("<HH", 2, 2) + bytes([0x80, 0, 0])
       + bytes([255,0,0, 0,255,0])
       + b"\x21\xF9\x04\x01\x00\x00\x00\x00"
       + b"\x2C\x00\x00\x00\x00\x02\x00\x02\x00\x00\x02\x02\x44\x01\x00\x3B")
open("testfiles/test.gif","wb").write(gif)

# SVG
svg = b'<?xml version="1.0"?><svg width="120" height="80" viewBox="0 0 120 80" version="1.1" xmlns="http://www.w3.org/2000/svg"></svg>'
open("testfiles/test.svg","wb").write(svg)

# MP3: ID3v2.3 + one MPEG1 L3 frame header
def synchsafe(n):
    return bytes([(n>>21)&0x7f,(n>>14)&0x7f,(n>>7)&0x7f,n&0x7f])
def frame(fid, payload):
    return fid + struct.pack(">I", len(payload)) + b"\x00\x00" + payload
tits = b"\x00" + b"Hello World"
tpe  = b"\x00" + b"Unit Test"
talb = b"\x00" + b"Binary Album"
tcon = b"\x00" + b"Rock"
body = frame(b"TIT2", tits) + frame(b"TPE1", tpe) + frame(b"TALB", talb) + frame(b"TCON", tcon)
id3 = b"ID3" + bytes([3,0,0]) + synchsafe(len(body)) + body
# MPEG-1 Layer3 128kbps 44100 stereo: FF FB 90 00
mp3 = id3 + b"\xff\xfb\x90\x00" + b"\x00"*417 + b"TAG" + b"Hello World".ljust(30,b"\0")+b"Unit Test".ljust(30,b"\0")+b"Binary Album".ljust(30,b"\0")+b"2024"+b"a"*29+b"\x00"
open("testfiles/test.mp3","wb").write(mp3)

# MP4: ftyp + moov(mvhd, trak(tkhd))
mac = 0xE9C0A000
mvhd = bytes([0,0,0,0]) + struct.pack(">IIII", mac, mac, 1000, 12345) + b"\x00"*60
tkhd = bytes([0,0,0,3]) + struct.pack(">II", mac, mac) + b"\x00"*8 + struct.pack(">II", 0, 0) + b"\x00"*48 + struct.pack(">II", 1920<<16, 1080<<16)
def box(t, b): return struct.pack(">I", 8+len(b)) + t + b
moov_body = box(b"mvhd", mvhd) + box(b"trak", box(b"tkhd", tkhd))
mp4 = box(b"ftyp", b"isom" + struct.pack(">I", 512) + b"isomiso2avc1mp41") + box(b"moov", moov_body)
open("testfiles/test.mp4","wb").write(mp4)

# TIFF with IFD0 + ExifIFD + GPS (little-endian)
# layout: header(8) | IFD0 | exifIFD | gpsIFD | string data
def entry(tag, typ, count, val):
    return struct.pack("<HHI", tag, typ, count) + val
strdata = b"TestCam\x00" + b"Test Model X\x00" + b"2024:06:01 12:00:00\x00"
# offsets computed after sizes known
ifd0_n, exif_n, gps_n = 6, 4, 4
ifd0_size = 2 + ifd0_n*12 + 4
exif_size = 2 + exif_n*12 + 4
gps_size  = 2 + gps_n*12 + 4
base = 8
ifd0 = base
exif = ifd0 + ifd0_size
gps = exif + exif_size
strs = gps + gps_size
o1 = strs; o2 = o1+8; o3 = o2+13
entries0 = (
    entry(0x0100, 3, 1, struct.pack("<HH", 640, 0)) +
    entry(0x0101, 3, 1, struct.pack("<HH", 480, 0)) +
    entry(0x0112, 3, 1, struct.pack("<HH", 6, 0)) +
    entry(0x0131, 2, 20, struct.pack("<I", o3)) +
    entry(0x8769, 4, 1, struct.pack("<I", exif)) +
    entry(0x8825, 4, 1, struct.pack("<I", gps))
)
ifd0_bytes = struct.pack("<H", ifd0_n) + entries0 + struct.pack("<I", 0)
entriesE = (
    entry(0x010F, 2, 8, struct.pack("<I", o1)) +
    entry(0x0110, 2, 13, struct.pack("<I", o2)) +
    entry(0x920A, 5, 1, struct.pack("<I", strs+48)) +
    entry(0xA002, 4, 1, struct.pack("<I", 640))
)
rat = struct.pack("<II", 50, 10)
entriesG = (
    entry(0x0001, 2, 2, b"N\x00\x00\x00") +
    entry(0x0002, 5, 3, struct.pack("<I", strs+64)) +
    entry(0x0003, 2, 2, b"E\x00\x00\x00") +
    entry(0x0004, 5, 3, struct.pack("<I", strs+88))
)
exif_bytes = struct.pack("<H", exif_n) + entriesE + struct.pack("<I", 0)
gps_bytes = struct.pack("<H", gps_n) + entriesG + struct.pack("<I", 0)
# string area: o1(8) o2(15) o3(20) then rationals at strs+40, +48, +72
area = b"TestCam\x00" + b"Test Model X\x00" + b"2024:06:01 12:00:00\x00"
pad = strs+48 - (strs+len(area))
area += b"\x00"*pad
area += struct.pack("<II", 50, 10)                      # focal 5.0
area += b"\x00"*(strs+64-(strs+len(area)))
area += struct.pack("<IIIIII", 48,1, 22,1, 3000,60)      # lat 48:22:30
area += b"\x00"*(strs+88-(strs+len(area)))
area += struct.pack("<IIIIII", 16,1, 37,1, 0,1)          # lon 16:37:0
tiff = b"II*\x00" + struct.pack("<I", ifd0) + ifd0_bytes + exif_bytes + gps_bytes + area
open("testfiles/test.tiff","wb").write(tiff)

# FLAC: fLaC + STREAMINFO + VORBIS_COMMENT
sr, ch, bits, total = 44100, 2, 16, 441000
b12 = ((sr & 0xF) << 4) | ((ch-1) << 1) | ((bits-1) >> 4 & 1)
b13 = (((bits-1) & 0xF) << 4) | ((total >> 32) & 0xF)
si = (struct.pack(">HH", 4096, 4096)
      + struct.pack(">I", 8969)[1:] + struct.pack(">I", 19770)[1:]
      + bytes([sr >> 12, (sr >> 4) & 0xFF, b12, b13])
      + struct.pack(">I", total & 0xFFFFFFFF) + bytes(range(16)))
vendor = b"reference libFLAC 1.4.3 20230623"
comments = [b"TITLE=Test Song", b"ARTIST=Tester", b"GENRE=Punk"]
vc = struct.pack("<I", len(vendor)) + vendor + struct.pack("<I", len(comments))
for c in comments: vc += struct.pack("<I", len(c)) + c
flac = b"fLaC" + bytes([0x00]) + struct.pack(">I", len(si))[1:] + si \
       + bytes([0x84]) + struct.pack(">I", len(vc))[1:] + vc
open("testfiles/test.flac","wb").write(flac)

# Opus (Ogg container)
def ogg_page(body, granule=0, serial=1, seq=0, htype=0):
    segs = []
    n = len(body)
    while n >= 255: segs.append(255); n -= 255
    segs.append(n)
    return (b"OggS" + bytes([0, htype]) + struct.pack("<q", granule) +
            struct.pack("<II", serial, seq) + b"\x00\x00\x00\x00" +
            bytes([len(segs)]) + bytes(segs) + body)
opus_head = (b"OpusHead" + bytes([1, 2]) + struct.pack("<H", 3840) +
             struct.pack("<I", 48000) + struct.pack("<h", 0) + bytes([0]))
vendor = b"libopus"
cmts = [b"TITLE=Test Opus", b"ARTIST=Tester"]
opus_tags = b"OpusTags" + struct.pack("<I", len(vendor)) + vendor + struct.pack("<I", len(cmts))
for c in cmts: opus_tags += struct.pack("<I", len(c)) + c
opus = (ogg_page(opus_head, 0, 1, 0, 0x02) + ogg_page(opus_tags, 0, 1, 1, 0x04) +
        ogg_page(b"\xf8\xff\xfe" * 20, 483840, 1, 2, 0x04))
open("testfiles/test.opus","wb").write(opus)

# M4A with ilst tags
def tag_atom(name, payload):
    data = struct.pack(">I", 16+len(payload)) + b"data" + struct.pack(">II", 1, 0) + payload
    return struct.pack(">I", 8+len(data)) + name + data
ilst_body = (tag_atom(b"\xa9nam", b"Test Song") + tag_atom(b"\xa9ART", b"Tester") +
             tag_atom(b"\xa9alb", b"Binary Album") + tag_atom(b"trkn", b"\x00\x00\x00\x03\x00\x0c\x00\x00") +
             tag_atom(b"covr", b"\xff\xd8\xff\xe0junkjpeg\xff\xd9"))
meta = bytes([0,0,0,0]) + box(b"hdlr", b"\x00"*16 + b"mdirappl" + b"\x00\x00\x00\x00\x00\x00\x00\x00") + box(b"ilst", ilst_body)
mvhd_a = bytes([0,0,0,0]) + struct.pack(">IIII", mac, mac, 1000, 10500) + b"\x00"*60
moov_a = box(b"mvhd", mvhd_a) + box(b"udta", box(b"meta", meta))
m4a = box(b"ftyp", b"M4A " + struct.pack(">I", 0) + b"M4A mp42isom") + box(b"moov", moov_a)
open("testfiles/test.m4a","wb").write(m4a)

print("wrote", os.listdir("testfiles"))

