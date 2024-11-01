import argparse
import json
import sys

import torch
from transformers import pipeline


def summarize_text(text):
    # Check if a GPU is available
    device = 0 if torch.cuda.is_available() else -1

    # Create the summarization pipeline with the appropriate device
    summarizer = pipeline(
        "summarization", model="facebook/bart-large-cnn", device=device
    )
    summary = summarizer(text, max_length=150, min_length=30, do_sample=False)
    return summary[0]["summary_text"] if summary else None


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Summarize text using a transformer model."
    )
    parser.add_argument("text", type=str, help="The text to summarize")
    args = parser.parse_args()

    response = {"summary": None, "error": None}

    try:
        summary = summarize_text(args.text)
        if summary:
            response["summary"] = summary
        else:
            response["error"] = "No summary could be generated."
            sys.exit(1)
    except Exception as e:
        response["error"] = f"An error occurred: {e}"
        sys.exit(1)
    finally:
        print(json.dumps(response))
