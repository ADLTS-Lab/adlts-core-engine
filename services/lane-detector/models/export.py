import torch
# IMPORT YOUR MODEL ARCHITECTURE HERE
# For example: from model.architecture import UltrafastLaneDetectionModel

def export_to_onnx():
    # 1. Initialize your model architecture
    model = YOUR_MODEL_CLASS_NAME(...) # e.g. UltrafastLaneDetectionModel()
    
    # 2. Load the .pth weights into the model
    model.load_state_dict(torch.load("best_source_model.pth", map_location="cpu"))
    
    # 3. Put the model in evaluation mode
    model.eval()

    # 4. Create a dummy input tensor showing the exact shape the model expects
    # (For adlts-core-engine passing through ONNX, we use 1 batch, 3 channels, 288 height, 800 width)
    dummy_input = torch.randn(1, 3, 288, 800, device="cpu")

    # 5. Export to ONNX!
    torch.onnx.export(
        model,
        dummy_input,
        "best_source_model.onnx",            # Output file name
        export_params=True,                  # Store the trained weights
        opset_version=11,                    # ONNX standard version (11 is generally very safe)
        do_constant_folding=True,            # Optimize the graph
        input_names=["input"],               # Name of input tensor
        output_names=["output"],             # Name of output tensor
        dynamic_axes={                       # Allow batch size to be dynamic
            "input": {0: "batch_size"},
            "output": {0: "batch_size"}
        }
    )
    print("Successfully exported best_source_model.onnx!")

if __name__ == "__main__":
    export_to_onnx()
