import logging

import tiktoken

logger = logging.getLogger(__name__)


def get_encoder_for_model(model):
    if model == "gpt-3.5-turbo":
        return tiktoken.encoding_for_model("gpt-3.5-turbo-0301")

    if model == "gpt-4":
        return tiktoken.encoding_for_model("gpt-4-0314")

    raise NotImplementedError(
        f"get_encoder_for_model() is not implemented for model {model}."
        + " See https://github.com/openai/openai-python/blob/main/chatml.md"
        + "for information on how messages are converted to tokens."
    )


def count_tokens_for_message(encoder, messages):
    """Returns the number of tokens used by a list of messages."""
    # every message follows <|start|>{role/name}\n{content}<|end|>\n
    tokens_per_message = 4
    tokens_per_name = -1  # if there's a name, the role is omitted
    num_tokens = 0

    for message in messages:
        num_tokens += tokens_per_message
        for key, value in message.items():
            num_tokens += len(encoder.encode(value))
            if key == "name":
                num_tokens += tokens_per_name
    num_tokens += 3  # every reply is primed with <|start|>assistant<|message|>
    return num_tokens
