"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(_runtime_version.Domain.PUBLIC, 5, 29, 0, '', 'parser.proto')
_sym_db = _symbol_database.Default()
DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x0cparser.proto\x12\x06parser\"N\n\x0c\x43onvertChunk\x12#\n\x04meta\x18\x01 \x01(\x0b\x32\x13.parser.ConvertMetaH\x00\x12\x0e\n\x04\x64\x61ta\x18\x02 \x01(\x0cH\x00\x42\t\n\x07payload\"3\n\x0b\x43onvertMeta\x12\x10\n\x08\x66ilename\x18\x01 \x01(\t\x12\x12\n\nrequest_id\x18\x02 \x01(\t\"4\n\x11\x43onvertURLRequest\x12\x0b\n\x03url\x18\x01 \x01(\t\x12\x12\n\nrequest_id\x18\x02 \x01(\t\"\x8a\x01\n\rConvertResult\x12&\n\x06status\x18\x01 \x01(\x0b\x32\x14.parser.StatusUpdateH\x00\x12\x18\n\x0emarkdown_chunk\x18\x02 \x01(\x0cH\x00\x12,\n\x08metadata\x18\x03 \x01(\x0b\x32\x18.parser.DocumentMetadataH\x00\x42\t\n\x07payload\"@\n\x10\x44ocumentMetadata\x12\r\n\x05title\x18\x01 \x01(\t\x12\x0e\n\x06\x61uthor\x18\x02 \x01(\t\x12\r\n\x05\x63over\x18\x03 \x01(\x0c\"v\n\x0cStatusUpdate\x12\x0e\n\x06status\x18\x01 \x01(\t\x12\x12\n\npages_done\x18\x02 \x01(\x05\x12\x13\n\x0bpages_total\x18\x03 \x01(\x05\x12\r\n\x05\x65rror\x18\x04 \x01(\t\x12\r\n\x05stage\x18\x05 \x01(\t\x12\x0f\n\x07message\x18\x06 \x01(\t2\x95\x01\n\rParserService\x12\x42\n\x0f\x43onvertDocument\x12\x14.parser.ConvertChunk\x1a\x15.parser.ConvertResult(\x01\x30\x01\x12@\n\nConvertURL\x12\x19.parser.ConvertURLRequest\x1a\x15.parser.ConvertResult\x30\x01\x42\x37Z5github.com/GustavoCaso/folio/ui/internal/parser/protob\x06proto3')
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
    _globals['_CONVERTRESULT']._serialized_end = 350
    _globals['_DOCUMENTMETADATA']._serialized_start = 352
    _globals['_DOCUMENTMETADATA']._serialized_end = 416
    _globals['_STATUSUPDATE']._serialized_start = 418
    _globals['_STATUSUPDATE']._serialized_end = 536
    _globals['_PARSERSERVICE']._serialized_start = 539
    _globals['_PARSERSERVICE']._serialized_end = 688