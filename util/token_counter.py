import logging

import tiktoken

logger = logging.getLogger(__name__)


def get_encoder_for_model(model):
    if model == "gpt-3.5-turbo":
        return tiktoken.encoding_for_model("gpt-3.5-turbo-0301")

    if model == "gpt-4":
        return tiktoken.encoding_for_model("gpt-4-0314")

    return tiktoken.encoding_for_model(model)


def count_tokens_for_message(encoder, messages):
    """Returns the number of tokens used by a list of messages."""
    tokens_per_message = 8
    num_tokens = 0

    for message in messages:
        num_tokens += tokens_per_message
        for _, value in message.items():
            num_tokens += len(encoder.encode(value))

    return num_tokens
