from pathlib import Path
import struct, sys

# Build a native COFF .syso resource object so the Go linker owns the icon
# resource from link time. This avoids post-link PE mutation and matches what
# Windows Explorer/Shell expects from ordinary Windows applications.

def align(v, a):
    return (v + a - 1) // a * a

ico_path = Path(sys.argv[1] if len(sys.argv) > 1 else 'OF_Model_icon_round.ico')
out_path = Path(sys.argv[2] if len(sys.argv) > 2 else 'icon_windows_amd64.syso')
ib = ico_path.read_bytes()
reserved, typ, count = struct.unpack_from('<HHH', ib, 0)
if (reserved, typ) != (0, 1) or count < 1:
    raise SystemExit('invalid ico')
entries=[]
for i in range(count):
    o=6+i*16
    w,h,colors,resv,planes,bpp,size,off=struct.unpack_from('<BBBBHHII',ib,o)
    data=ib[off:off+size]
    if len(data) != size:
        raise SystemExit('truncated ico')
    entries.append((w,h,colors,resv,planes,bpp,size,data))

# Resource directory, with all offsets relative to start of .rsrc.
root_off=0
root_size=16+2*8
icon_type_off=root_size
icon_type_size=16+count*8
lang_dirs_off=icon_type_off+icon_type_size
lang_dir_size=24
icon_lang_offs=[lang_dirs_off+i*lang_dir_size for i in range(count)]
group_type_off=lang_dirs_off+count*lang_dir_size
group_type_size=16+8
group_lang_off=group_type_off+group_type_size
group_lang_size=24
data_entries_off=group_lang_off+group_lang_size
data_entry_count=count+1
data_entries_size=data_entry_count*16
data_off=align(data_entries_off+data_entries_size,4)

grp=bytearray(struct.pack('<HHH',0,1,count))
for i,(w,h,colors,resv,planes,bpp,size,data) in enumerate(entries,1):
    grp += struct.pack('<BBBBHHIH',w,h,colors,resv,planes,bpp,size,i)

blobs=[]
cur=data_off
for e in entries:
    cur=align(cur,4); blobs.append((cur,e[7])); cur += len(e[7])
cur=align(cur,4); group_blob=(cur,bytes(grp)); cur += len(grp)
raw=bytearray(align(cur,4))

def put_dir(off, id_entries):
    struct.pack_into('<IIHHHH',raw,off,0,0,0,0,0,len(id_entries))
    for i,(rid,target,isdir) in enumerate(id_entries):
        struct.pack_into('<II',raw,off+16+i*8,rid,target | (0x80000000 if isdir else 0))

put_dir(root_off,[(3,icon_type_off,True),(14,group_type_off,True)])
put_dir(icon_type_off,[(i+1,icon_lang_offs[i],True) for i in range(count)])
for i,lo in enumerate(icon_lang_offs):
    put_dir(lo,[(0x409,data_entries_off+i*16,False)])
put_dir(group_type_off,[(1,group_lang_off,True)])
put_dir(group_lang_off,[(0x409,data_entries_off+count*16,False)])

reloc_offsets=[]
for i,(doff,data) in enumerate(blobs):
    # Address field is section-relative here. COFF relocation will turn it into
    # an image-relative RVA (ADDR32NB) at link time.
    struct.pack_into('<IIII',raw,data_entries_off+i*16,doff,len(data),0,0)
    reloc_offsets.append(data_entries_off+i*16)
    raw[doff:doff+len(data)] = data

goff,gdata=group_blob
struct.pack_into('<IIII',raw,data_entries_off+count*16,goff,len(gdata),0,0)
reloc_offsets.append(data_entries_off+count*16)
raw[goff:goff+len(gdata)] = gdata

# AMD64 COFF object: header + one .rsrc section + raw data + relocations +
# one static section symbol + 4-byte empty string table.
file_header_size=20
section_header_size=40
raw_ptr=file_header_size+section_header_size
reloc_ptr=raw_ptr+len(raw)
reloc_count=len(reloc_offsets)
sym_ptr=reloc_ptr+reloc_count*10
num_symbols=1
coff=bytearray()
coff += struct.pack('<HHIIIHH',0x8664,1,0,sym_ptr,num_symbols,0,0x0004)
coff += struct.pack('<8sIIIIIIHHI',b'.rsrc\0\0\0',0,0,len(raw),raw_ptr,reloc_ptr,0,reloc_count,0,0xC0300040)
coff += raw
for off in reloc_offsets:
    coff += struct.pack('<IIH',off,0,0x0003)  # IMAGE_REL_AMD64_ADDR32NB
# Static section symbol .rsrc
coff += struct.pack('<8sIhHBB',b'.rsrc\0\0\0',0,1,0,3,0)
coff += struct.pack('<I',4)
out_path.write_bytes(coff)
print(f'wrote {out_path} ({len(coff)} bytes), frames={count}, relocations={reloc_count}')
