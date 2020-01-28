## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service enables users
to upload files (up to 64MB each) for unlimited retrieval within 24 hours.

Uploaded files are temporary in nature, they are automatically deleted after 24 hours.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `FileUploadEndpoint`, value being the URL
location that users visit to upload and retrieve files. The location should be a secret for intended users only -
make it difficult to guess.

Here is an example setup:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "FileUploadEndpoint": "/very-secret-file-upload-place",

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
1. In a web browser, navigate to `FileUploadEndpoint` of laitos web server.
2. Click "Choose file", after selecting a file click "Upload".
3. Observe successful message: "Uploaded successfully. Your file is available for 24 hours under name: abcdefghij.ext".
   Write down the file name on a piece of paper.
4. Visit `FileUploadEndpoint` within 24 hours, enter the file name into the text field and click "Download".

## Tips
- Make the URL location secure and hard to guess, it is the only way to secure this web service!
- The download button asks browser to use its default action on the downloaded file - if the file is a photo, then browsers will often 
  display the photo rather than displaying a download dialogue. You can save the photo by right-clicking the photo on a desktop computer,
  or long-press the photo on a tablet computer.
- The temporary storage place is located underneath directory `laitos-HandleFileUpload` in operationg system's temporary file directory.
  On many Linux systems, each system service gets their own private temporary files directory underneath `/tmp/`.
