import argparse
import json
import sys

import torch
from transformers import pipeline


def split_text(text, max_length):
    sentences = text.split(". ")
    chunks = []
    current_chunk = []
    current_length = 0

    for sentence in sentences:
        sentence_length = len(sentence.split())
        if current_length + sentence_length > max_length:
            chunks.append(". ".join(current_chunk) + ".")
            current_chunk = [sentence]
            current_length = sentence_length
        else:
            current_chunk.append(sentence)
            current_length += sentence_length

    if current_chunk:
        chunks.append(". ".join(current_chunk) + ".")

    return chunks


def summarize_text(text):
    # Check if a GPU is available
    device = 0 if torch.cuda.is_available() else -1

    # Create the summarization pipeline with the appropriate device
    summarizer = pipeline(
        "summarization", model="facebook/bart-large-cnn", device=device
    )

    # Add debugging information
    print(f"Text length: {len(text)}", file=sys.stderr)

    # Ensure the text is not empty
    if not text.strip():
        raise ValueError("Input text is empty")

    max_length = 512  # Maximum length for the model
    chunks = split_text(text, max_length)

    summaries = []
    for chunk in chunks:
        try:
            summary = summarizer(chunk, max_length=150, min_length=30, do_sample=False)
            print(f"Summary chunk: {summary}", file=sys.stderr)  # Debug print
            summaries.append(summary[0]["summary_text"])
        except Exception as e:
            print(f"Error summarizing chunk: {chunk[:100]}... - {e}", file=sys.stderr)
            raise

    # Combine the summaries into a single summary
    combined_summary = " ".join(summaries)

    # Add debugging information
    print(f"Summary: {combined_summary}", file=sys.stderr)

    return combined_summary


def main():
    parser = argparse.ArgumentParser(
        description="Summarize text using a transformer model."
    )
    parser.add_argument("text", type=str, help="The text to summarize")
    args = parser.parse_args()

    response = {"summary": None, "error": None, "model_name": "facebook/bart-large-cnn"}

    try:
        summary = summarize_text(args.text)
        response["summary"] = summary
    except Exception as e:
        response["error"] = f"An error occurred: {e}"
        print(f"An error occurred: {e}", file=sys.stderr)
        sys.exit(1)
    finally:
        print(json.dumps(response))


if __name__ == "__main__":
    main()
