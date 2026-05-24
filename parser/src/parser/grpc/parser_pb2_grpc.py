"""Client and server classes corresponding to protobuf-defined services."""
import grpc
import warnings
from . import parser_pb2 as parser__pb2
GRPC_GENERATED_VERSION = '1.71.2'
GRPC_VERSION = grpc.__version__
_version_not_supported = False
try:
    from grpc._utilities import first_version_is_lower
    _version_not_supported = first_version_is_lower(GRPC_VERSION, GRPC_GENERATED_VERSION)
except ImportError:
    _version_not_supported = True
if _version_not_supported:
    raise RuntimeError(f'The grpc package installed is at version {GRPC_VERSION},' + f' but the generated code in parser_pb2_grpc.py depends on' + f' grpcio>={GRPC_GENERATED_VERSION}.' + f' Please upgrade your grpc module to grpcio>={GRPC_GENERATED_VERSION}' + f' or downgrade your generated code using grpcio-tools<={GRPC_VERSION}.')

class ParserServiceStub(object):
    """ParserService is stateless. No state is retained after a stream closes.
    """

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.ConvertDocument = channel.stream_stream('/parser.ParserService/ConvertDocument', request_serializer=parser__pb2.ConvertChunk.SerializeToString, response_deserializer=parser__pb2.ConvertResult.FromString, _registered_method=True)
        self.ConvertURL = channel.unary_stream('/parser.ParserService/ConvertURL', request_serializer=parser__pb2.ConvertURLRequest.SerializeToString, response_deserializer=parser__pb2.ConvertResult.FromString, _registered_method=True)

class ParserServiceServicer(object):
    """ParserService is stateless. No state is retained after a stream closes.
    """

    def ConvertDocument(self, request_iterator, context):
        """ConvertDocument streams file bytes from the caller and streams results back.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def ConvertURL(self, request, context):
        """ConvertURL fetches a document from a URL and streams results back.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

def add_ParserServiceServicer_to_server(servicer, server):
    rpc_method_handlers = {'ConvertDocument': grpc.stream_stream_rpc_method_handler(servicer.ConvertDocument, request_deserializer=parser__pb2.ConvertChunk.FromString, response_serializer=parser__pb2.ConvertResult.SerializeToString), 'ConvertURL': grpc.unary_stream_rpc_method_handler(servicer.ConvertURL, request_deserializer=parser__pb2.ConvertURLRequest.FromString, response_serializer=parser__pb2.ConvertResult.SerializeToString)}
    generic_handler = grpc.method_handlers_generic_handler('parser.ParserService', rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))
    server.add_registered_method_handlers('parser.ParserService', rpc_method_handlers)

class ParserService(object):
    """ParserService is stateless. No state is retained after a stream closes.
    """

    @staticmethod
    def ConvertDocument(request_iterator, target, options=(), channel_credentials=None, call_credentials=None, insecure=False, compression=None, wait_for_ready=None, timeout=None, metadata=None):
        return grpc.experimental.stream_stream(request_iterator, target, '/parser.ParserService/ConvertDocument', parser__pb2.ConvertChunk.SerializeToString, parser__pb2.ConvertResult.FromString, options, channel_credentials, insecure, call_credentials, compression, wait_for_ready, timeout, metadata, _registered_method=True)

    @staticmethod
    def ConvertURL(request, target, options=(), channel_credentials=None, call_credentials=None, insecure=False, compression=None, wait_for_ready=None, timeout=None, metadata=None):
        return grpc.experimental.unary_stream(request, target, '/parser.ParserService/ConvertURL', parser__pb2.ConvertURLRequest.SerializeToString, parser__pb2.ConvertResult.FromString, options, channel_credentials, insecure, call_credentials, compression, wait_for_ready, timeout, metadata, _registered_method=True)