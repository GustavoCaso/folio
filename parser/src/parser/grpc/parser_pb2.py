"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(_runtime_version.Domain.PUBLIC, 5, 29, 0, '', 'parser.proto')
_sym_db = _symbol_database.Default()
DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x0cparser.proto\x12\x06parser"N\n\x0cConvertChunk\x12#\n\x04meta\x18\x01 \x01(\x0b2\x13.parser.ConvertMetaH\x00\x12\x0e\n\x04data\x18\x02 \x01(\x0cH\x00B\t\n\x07payload"3\n\x0bConvertMeta\x12\x10\n\x08filename\x18\x01 \x01(\t\x12\x12\n\nrequest_id\x18\x02 \x01(\t"4\n\x11ConvertURLRequest\x12\x0b\n\x03url\x18\x01 \x01(\t\x12\x12\n\nrequest_id\x18\x02 \x01(\t"\xb5\x01\n\rConvertResult\x12&\n\x06status\x18\x01 \x01(\x0b2\x14.parser.StatusUpdateH\x00\x12\x18\n\x0emarkdown_chunk\x18\x02 \x01(\x0cH\x00\x12,\n\x08metadata\x18\x03 \x01(\x0b2\x18.parser.DocumentMetadataH\x00\x12)\n\x0bimage_chunk\x18\x04 \x01(\x0b2\x12.parser.ImageChunkH\x00B\t\n\x07payload",\n\nImageChunk\x12\x10\n\x08filename\x18\x01 \x01(\t\x12\x0c\n\x04data\x18\x02 \x01(\x0c"@\n\x10DocumentMetadata\x12\r\n\x05title\x18\x01 \x01(\t\x12\x0e\n\x06author\x18\x02 \x01(\t\x12\r\n\x05cover\x18\x03 \x01(\x0c"v\n\x0cStatusUpdate\x12\x0e\n\x06status\x18\x01 \x01(\t\x12\x12\n\npages_done\x18\x02 \x01(\x05\x12\x13\n\x0bpages_total\x18\x03 \x01(\x05\x12\r\n\x05error\x18\x04 \x01(\t\x12\r\n\x05stage\x18\x05 \x01(\t\x12\x0f\n\x07message\x18\x06 \x01(\t2\x95\x01\n\rParserService\x12B\n\x0fConvertDocument\x12\x14.parser.ConvertChunk\x1a\x15.parser.ConvertResult(\x010\x01\x12@\n\nConvertURL\x12\x19.parser.ConvertURLRequest\x1a\x15.parser.ConvertResult0\x01B7Z5github.com/GustavoCaso/folio/ui/internal/parser/protob\x06proto3')
_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'parser_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
    _globals['DESCRIPTOR']._loaded_options = None
    _globals['DESCRIPTOR']._serialized_options = b'Z5github.com/GustavoCaso/folio/ui/internal/parser/proto'
    _globals['_CONVERTCHUNK']._serialized_start = 24
    _globals['_CONVERTCHUNK']._serialized_end = 102
    _globals['_CONVERTMETA']._serialized_start = 104
    _globals['_CONVERTMETA']._serialized_end = 155
    _globals['_CONVERTURLREQUEST']._serialized_start = 157
    _globals['_CONVERTURLREQUEST']._serialized_end = 209
    _globals['_CONVERTRESULT']._serialized_start = 212
    _globals['_CONVERTRESULT']._serialized_end = 393
    _globals['_IMAGECHUNK']._serialized_start = 395
    _globals['_IMAGECHUNK']._serialized_end = 439
    _globals['_DOCUMENTMETADATA']._serialized_start = 441
    _globals['_DOCUMENTMETADATA']._serialized_end = 505
    _globals['_STATUSUPDATE']._serialized_start = 507
    _globals['_STATUSUPDATE']._serialized_end = 625
    _globals['_PARSERSERVICE']._serialized_start = 628
    _globals['_PARSERSERVICE']._serialized_end = 777