from pathlib import Path
import struct, subprocess, sys, shutil

exe = Path(sys.argv[1])
ico = Path(sys.argv[2])
out = Path(sys.argv[3])

def align(v,a): return (v+a-1)//a*a

# PE basics
b = exe.read_bytes()
pe = struct.unpack_from('<I',b,0x3c)[0]
if b[pe:pe+4] != b'PE\0\0': raise SystemExit('not PE')
num = struct.unpack_from('<H',b,pe+6)[0]
opt_size = struct.unpack_from('<H',b,pe+20)[0]
opt = pe+24
magic = struct.unpack_from('<H',b,opt)[0]
if magic != 0x20b: raise SystemExit('expected PE32+')
image_base = struct.unpack_from('<Q',b,opt+24)[0]
sect_align = struct.unpack_from('<I',b,opt+32)[0]
size_image = struct.unpack_from('<I',b,opt+56)[0]
rsrc_rva = align(size_image, sect_align)
rsrc_vma = image_base + rsrc_rva

# ICO entries
ib = ico.read_bytes()
res,typ,count = struct.unpack_from('<HHH',ib,0)
if (res,typ)!=(0,1) or count<1: raise SystemExit('invalid ico')
entries=[]
for i in range(count):
    o=6+i*16
    w,h,colors,resv,planes,bpp,size,off=struct.unpack_from('<BBBBHHII',ib,o)
    data=ib[off:off+size]
    entries.append((w,h,colors,resv,planes,bpp,size,data))

# Resource tree layout
# root: types RT_ICON(3), RT_GROUP_ICON(14)
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

# group data
grp=bytearray(struct.pack('<HHH',0,1,count))
for i,(w,h,colors,resv,planes,bpp,size,data) in enumerate(entries,1):
    grp += struct.pack('<BBBBHHIH',w,h,colors,resv,planes,bpp,size,i)

data_blobs=[]
cur=data_off
for e in entries:
    cur=align(cur,4); data_blobs.append((cur,e[7])); cur += len(e[7])
cur=align(cur,4); group_blob=(cur,bytes(grp)); cur += len(grp)
raw=bytearray(align(cur,4))

def put_dir(off, id_entries):
    struct.pack_into('<IIHHHH',raw,off,0,0,0,0,0,len(id_entries))
    for i,(rid,target,isdir) in enumerate(id_entries):
        val=target | (0x80000000 if isdir else 0)
        struct.pack_into('<II',raw,off+16+i*8,rid,val)

put_dir(root_off,[(3,icon_type_off,True),(14,group_type_off,True)])
put_dir(icon_type_off,[(i+1,icon_lang_offs[i],True) for i in range(count)])
for i,lo in enumerate(icon_lang_offs):
    put_dir(lo,[(0x409,data_entries_off+i*16,False)])
put_dir(group_type_off,[(1,group_lang_off,True)])
put_dir(group_lang_off,[(0x409,data_entries_off+count*16,False)])
for i,(doff,data) in enumerate(data_blobs):
    struct.pack_into('<IIII',raw,data_entries_off+i*16,rsrc_rva+doff,len(data),0,0)
    raw[doff:doff+len(data)] = data
goff,gdata=group_blob
struct.pack_into('<IIII',raw,data_entries_off+count*16,rsrc_rva+goff,len(gdata),0,0)
raw[goff:goff+len(gdata)] = gdata


# Add the resource section directly. No GNU objcopy or external toolchain is
# needed; identical embedding works in Linux and native Windows builds.
b = bytearray(b)
file_align = struct.unpack_from('<I', b, opt+36)[0]
section_table = opt + opt_size
header_end = section_table + (num+1)*40
first_raw = min(struct.unpack_from('<I', b, section_table+i*40+20)[0] for i in range(num))
if header_end > first_raw:
    raise SystemExit('No room for resource section header')
if struct.unpack_from('<I', b, opt+112+2*8)[0]:
    raise SystemExit('Resource directory already present: expected a fresh raw build')
raw_pointer = align(len(b), file_align)
raw_size = align(len(raw), file_align)
b.extend(b'\0' * (raw_pointer-len(b)))
b.extend(raw)
b.extend(b'\0' * (raw_size-len(raw)))
section_off = section_table + num*40
b[section_off:section_off+40] = struct.pack('<8sIIIIIIHHI', b'.rsrc\0\0\0', len(raw), rsrc_rva, raw_size, raw_pointer, 0, 0, 0, 0, 0x40000040)
struct.pack_into('<H', b, pe+6, num+1)
struct.pack_into('<I', b, opt+8, struct.unpack_from('<I', b, opt+8)[0]+raw_size)
struct.pack_into('<I', b, opt+56, align(rsrc_rva+len(raw), sect_align))
struct.pack_into('<II', b, opt+112+2*8, rsrc_rva, len(raw))
struct.pack_into('<I', b, opt+64, 0) # linker checksum invalidated by resources
out.write_bytes(b)
print(f'embedded {count} approved icon images; RVA=0x{rsrc_rva:x}; raw={len(raw)}; pure PE writer')
