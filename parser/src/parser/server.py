import asyncio
import logging
import os

import grpc

from parser.grpc import parser_pb2_grpc
from parser.servicer import ParserServicer

logger = logging.getLogger(__name__)


async def _serve(port: int, num_workers: int) -> None:
    servicer = ParserServicer(num_workers=num_workers)

    server = grpc.aio.server()
    parser_pb2_grpc.add_ParserServiceServicer_to_server(servicer, server)
    server.add_insecure_port(f"[::]:{port}")
    await server.start()
    logger.info("Parser gRPC server listening on port %d with %d workers", port, num_workers)

    try:
        await server.wait_for_termination()
    finally:
        servicer.shutdown()


def serve() -> None:
    logging.basicConfig(level=logging.INFO)
    port = int(os.environ.get("GRPC_PORT", "50051"))
    num_workers = int(os.environ.get("NUM_WORKERS", "2"))
    asyncio.run(_serve(port, num_workers))
