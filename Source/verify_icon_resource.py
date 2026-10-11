from pathlib import Path
import hashlib, struct, sys

if len(sys.argv) < 2:
    raise SystemExit('usage: verify_icon_resource.py <exe> [expected.ico]')
exe = Path(sys.argv[1])
b = exe.read_bytes()
pe = struct.unpack_from('<I', b, 0x3C)[0]
if b[pe:pe+4] != b'PE\0\0':
    raise SystemExit('ICON CHECK FAIL: invalid PE')
opt = pe + 24
if struct.unpack_from('<H', b, opt)[0] != 0x20B:
    raise SystemExit('ICON CHECK FAIL: expected PE32+')
rva, size = struct.unpack_from('<II', b, opt + 112 + 2*8)
if not rva or size < 1024:
    raise SystemExit('ICON CHECK FAIL: resource directory missing')
sections = struct.unpack_from('<H', b, pe + 6)[0]
opt_size = struct.unpack_from('<H', b, pe + 20)[0]
section_table = opt + opt_size
section_rows = []
for i in range(sections):
    off = section_table + i*40
    name = b[off:off+8].rstrip(b'\0')
    virtual_size, virtual_address, raw_size, raw_ptr = struct.unpack_from('<IIII', b, off+8)
    section_rows.append((name, virtual_address, max(virtual_size, raw_size), raw_ptr, raw_size))
if b'.rsrc' not in [x[0] for x in section_rows]:
    raise SystemExit('ICON CHECK FAIL: .rsrc section missing')

def rva_to_off(value):
    for _, va, span, raw_ptr, raw_size in section_rows:
        if va <= value < va + span:
            delta = value - va
            if delta >= raw_size:
                raise SystemExit('ICON CHECK FAIL: resource RVA points outside raw section')
            return raw_ptr + delta
    raise SystemExit(f'ICON CHECK FAIL: cannot map RVA 0x{value:x}')

rsrc_off = rva_to_off(rva)

def read_dir(rel):
    off = rsrc_off + rel
    if off + 16 > len(b):
        raise SystemExit('ICON CHECK FAIL: truncated resource directory')
    named, ids = struct.unpack_from('<HH', b, off+12)
    out = {}
    for i in range(named + ids):
        name_or_id, target = struct.unpack_from('<II', b, off+16+i*8)
        if name_or_id & 0x80000000:
            continue
        rid = name_or_id & 0xFFFF
        out[rid] = (target & 0x7FFFFFFF, bool(target & 0x80000000))
    return out

def leaf_data(rel):
    d = read_dir(rel)
    if not d:
        raise SystemExit('ICON CHECK FAIL: empty resource directory')
    # This package uses one language (0x409); tolerate another single language.
    child_rel, is_dir = next(iter(d.values()))
    if is_dir:
        d2 = read_dir(child_rel)
        if not d2:
            raise SystemExit('ICON CHECK FAIL: empty language directory')
        child_rel, is_dir = next(iter(d2.values()))
    if is_dir:
        raise SystemExit('ICON CHECK FAIL: unexpected extra resource nesting')
    data_entry_off = rsrc_off + child_rel
    data_rva, data_size = struct.unpack_from('<II', b, data_entry_off)
    data_off = rva_to_off(data_rva)
    return b[data_off:data_off+data_size]

root = read_dir(0)
if 3 not in root or 14 not in root:
    raise SystemExit('ICON CHECK FAIL: RT_ICON/RT_GROUP_ICON tree missing')
icon_dir_rel, icon_dir_flag = root[3]
group_dir_rel, group_dir_flag = root[14]
if not icon_dir_flag or not group_dir_flag:
    raise SystemExit('ICON CHECK FAIL: icon resource types are not directories')
icon_nodes = read_dir(icon_dir_rel)
group_nodes = read_dir(group_dir_rel)
if not icon_nodes or not group_nodes:
    raise SystemExit('ICON CHECK FAIL: icon/group resource nodes missing')
if 1 not in group_nodes:
    raise SystemExit('ICON CHECK FAIL: primary group icon ID 1 missing')
group_data = leaf_data(group_nodes[1][0])
if len(group_data) < 6:
    raise SystemExit('ICON CHECK FAIL: group icon data truncated')
g_reserved, g_type, g_count = struct.unpack_from('<HHH', group_data, 0)
if (g_reserved, g_type) != (0, 1) or g_count < 1:
    raise SystemExit('ICON CHECK FAIL: invalid group icon header')
if len(group_data) < 6 + g_count*14:
    raise SystemExit('ICON CHECK FAIL: group icon entries truncated')
group_entries = []
for i in range(g_count):
    w, h, colors, reserved, planes, bpp, data_size, icon_id = struct.unpack_from('<BBBBHHIH', group_data, 6+i*14)
    group_entries.append((w, h, colors, reserved, planes, bpp, data_size, icon_id))
if any(e[-1] not in icon_nodes for e in group_entries):
    raise SystemExit('ICON CHECK FAIL: group icon references missing RT_ICON IDs')

if len(sys.argv) >= 3:
    ico_path = Path(sys.argv[2])
    ico = ico_path.read_bytes()
    if len(ico) < 6:
        raise SystemExit('ICON CHECK FAIL: expected ICO is invalid')
    reserved, typ, count = struct.unpack_from('<HHH', ico, 0)
    if reserved != 0 or typ != 1 or count < 1:
        raise SystemExit('ICON CHECK FAIL: expected ICO header is invalid')
    payloads = []
    expected_sizes = []
    for i in range(count):
        off = 6 + i*16
        _, _, _, _, _, _, data_size, data_off = struct.unpack_from('<BBBBHHII', ico, off)
        payload = ico[data_off:data_off+data_size]
        if len(payload) != data_size:
            raise SystemExit('ICON CHECK FAIL: expected ICO payload is truncated')
        payloads.append(payload)
        expected_sizes.append(data_size)
    missing = [i for i, payload in enumerate(payloads) if payload not in b]
    if missing:
        raise SystemExit(f'ICON CHECK FAIL: executable does not contain approved icon payload(s): {missing}')
    if g_count != count or len(icon_nodes) != count:
        raise SystemExit(f'ICON CHECK FAIL: resource tree frame count mismatch: group={g_count}, icons={len(icon_nodes)}, expected={count}')
    resource_sizes = [entry[6] for entry in group_entries]
    if resource_sizes != expected_sizes:
        raise SystemExit('ICON CHECK FAIL: group icon frame sizes do not match approved ICO')
    sha = hashlib.sha256(ico).hexdigest()
    print(f'PASS: approved icon resource tree is valid; frames={count}; expected_sha256={sha}')

print(f'PASS: embedded application icon (.rsrc RVA=0x{rva:x}, size={size}; RT_ICON={len(icon_nodes)}, RT_GROUP_ICON={len(group_nodes)})')
