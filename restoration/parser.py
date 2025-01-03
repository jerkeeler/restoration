import gzip
import io
import logging
import string
import struct
import zlib
from dataclasses import dataclass, field
from functools import cached_property

from restoration.xceptions import NodeNotFound

logger = logging.getLogger(__name__)

OUTER_HIERARCHY_START_OFFSET = 257
DATA_OFFSET = 6
UPPERCASE_ASSCII = set(bytes(string.ascii_uppercase, "utf-8"))


@dataclass
class Node:
    token: str
    offset: int
    size: int
    parsed: bool = False
    parent: "Node | None" = None
    children: list["Node"] = field(default_factory=list)

    def __str__(self) -> str:
        """
        Making it easer to debug.
        """
        return (
            f"{self.path} -- offset={self.offset}, end_offset={self.end_offset} "
            f"size={self.size}, children={len(self.children)}"
        )

    @cached_property
    def end_offset(self):
        return self.offset + self.size + DATA_OFFSET

    @cached_property
    def path(self) -> str:
        """
        A string representing the "path" of the node based on its and its parents tokens. For example, if this node
        has a token of JK and the parent is AB the path would be AB/JK.
        """
        if self.parent is None:
            return self.token
        return f"{self.parent.path}/{self.token}"

    def get_children(self, path: list[str]) -> list["Node"]:
        """
        Get the children of this node that match the give path. Some paths have more than one node. For example, there
        are multiple nodes with the XN/XN/XN path.
        """
        if not path:
            return [self]

        nodes = []
        for child in self.children:
            if child.token == path[0]:
                nodes.extend(child.get_children(path[1:]))

        return nodes


def decompressl33t(stream: io.BufferedReader | gzip.GzipFile) -> io.BytesIO:
    # Read the first 4 bytes of the stream and check if it is l33t encoded
    header = stream.read(4)
    if header != b"l33t":
        raise ValueError("Invalid header. Expecting 'l33t'")

    decompress = zlib.decompressobj()
    # Read the length of the compressed data, need to read so that the rest can be
    # decompressed with one read operation
    struct.unpack("<i", stream.read(4))[0]
    return io.BytesIO(decompress.decompress(stream.read()))


def parse_rec(stream: io.BufferedReader | gzip.GzipFile) -> tuple[io.BytesIO, Node]:
    decompressed_data = decompressl33t(stream)
    decompressed_data.seek(OUTER_HIERARCHY_START_OFFSET)
    bs = decompressed_data.read(2)
    two_letter_code = bs.decode("utf-8")
    data_len = struct.unpack("<H", decompressed_data.read(2))[0]
    # 6 bytes of padding for no reason? Don't think this is needed
    # decompressed_data.read(DATA_OFFSET)
    root_node = Node(
        token=two_letter_code,
        offset=OUTER_HIERARCHY_START_OFFSET,
        size=data_len,
    )
    logger.debug(two_letter_code)
    logger.debug(data_len)
    recursive_create_tree(root_node, decompressed_data)
    read_build_string(root_node, decompressed_data)
    parse_profile_keys(root_node, decompressed_data)
    return decompressed_data, root_node


def recursive_create_tree(
    parent_node: Node,
    decompressed_data: io.BytesIO,
) -> None:
    """
    Recursively build up the metadata tree using a breadth first search approach.
    """
    decompressed_data.seek(parent_node.offset)
    # Skip the first two bytes to start the children search since those are the node token
    position = parent_node.offset + 2
    while position < parent_node.end_offset:
        next_node_loc = find_two_letter_seq(
            decompressed_data,
            offset=position,
            upper_bound=parent_node.end_offset,
        )
        if next_node_loc == -1:
            break

        decompressed_data.seek(next_node_loc)
        token = decompressed_data.read(2).decode("utf-8")
        size = struct.unpack("<H", decompressed_data.read(2))[0]
        node = Node(
            token=token,
            offset=next_node_loc,
            size=size,
            parent=parent_node,
        )
        decompressed_data.seek(node.end_offset)
        parent_node.children.append(node)
        position = node.end_offset

    for child in parent_node.children:
        recursive_create_tree(child, decompressed_data)


def find_two_letter_seq(
    data: io.BytesIO,
    offset: int = 0,
    upper_bound: int | None = None,
) -> int:
    """
    Searches for the sequence b"<Uppercase ASCII><Uppercase ASCII>". Moves the reads to offset before
    searching and moves it back to offset once done searching.
    """
    data.seek(offset)
    position = offset

    while True:
        if upper_bound is not None and position >= upper_bound:
            break

        data.seek(position)
        # Note, there's a weird thing with python that if you have a byte string with
        # a single byte like b'A', it will be treated as an integer. But if you have
        # a byte string with two bytes like b'AB', it will be treated as a byte string.
        # It's very annoying. So you could change this to read a new character one at
        # a time, it would be more efficient, but I'm not annoyed to figure it out.
        byte1, byte2 = data.read(2)
        if byte1 in UPPERCASE_ASSCII and byte2 in UPPERCASE_ASSCII:
            data.seek(offset)
            return position

        position += 1

    data.seek(offset)
    return -1


def read_string(data: io.BytesIO, offset: int) -> tuple[str, int]:
    """
    Reads the utf-16 little endian encoded at the given offset. Strings are enocde such that the first 2 bytes
    are an unsigned integer indicating the number of characters in the string. The next 2 bytes are null padding.
    Then the string follows. Since the strings are unicode enocode each character takes up 2 bytes.
    For example a string might look like:
    \x02\x00\x00\x00H\x00e\x00l\x00l\x00o\x00
    """
    data.seek(offset)
    num_chars = struct.unpack("<H", data.read(2))[0]
    data.read(2)  # Skip 2 null padding bytes
    # Characters are defined with 2 bytes using utf-16 little endian encoding, so multiple by 2 to get the number
    # of bytes to read
    raw_string = data.read(num_chars * 2)
    s = raw_string.decode("utf-16-le")
    return s, offset + 4 + num_chars * 2


def read_build_string(root_node: Node, data: io.BytesIO) -> str:
    """
    Finds the FH node, then reads the string at the FH node offset to get the build information. There is other
    information in the FH node, but I don't know what it is.
    """
    children = root_node.get_children(["FH"])
    if not children:
        raise NodeNotFound("Could not find FH node! No build string!")
    if len(children) > 1:
        logger.warning("Found multiple FH nodes! Using the first one.")
    fh_node = children[0]
    position = fh_node.offset + 6  # skip the token and data length (4) + 2 null padding bytes
    data.seek(position)
    s, _ = read_string(data, position)
    logger.debug(f"build_string={s}")
    return s


def parse_profile_keys(root_node: Node, data: io.BytesIO) -> None:
    children = root_node.get_children(["MP", "ST"])
    if not children:
        raise NodeNotFound("Could not find ST node! No profile keys!")
    if len(children) > 1:
        logger.warning("Found multiple ST nodes! Using the first one.")
    st_node = children[0]
    # Skip the token and data length (4) + 6 null padding bytes
    position = st_node.offset + 10
    data.seek(position)
    num_keys = struct.unpack("<H", data.read(2))[0]
    logger.debug(f"{num_keys=}")
    position += 4  # Position + 2 for the num_keys read and skip 2 null padding bytes
    for _ in range(num_keys):
        keyname, new_position = read_string(data, position)
        keytype = struct.unpack("<H", data.read(2))[0]
        logger.debug(f"{keyname=}, {keytype=}, {position=}, {new_position=}")
        position = new_position + 2  # Skip the keytype and 4 null padding bytes
        if keytype == 10:
            # 10 is string
            s, new_pos = read_string(data, position)
            logger.debug(f"{s=}")
            position = new_pos + 2  # Skip 2 null padding bytes
        elif keytype == 2:
            # 2 is int
            data.seek(position + 2)  # Skip 2 null padding bytes
            i = struct.unpack("<H", data.read(2))[0]
            logger.debug(f"{i=}")
            position = position + 6  # Skip 2 for the int and 2 null padding bytes
        elif keytype == 6:
            # 6 is boolean
            data.seek(position)
            b = struct.unpack("<?", data.read(1))[0]
            logger.debug(f"{b=}")
            position = position + 3
        else:
            raise ValueError(f"Unknown keytype: {keytype}")
