package com.openfaas.function;

import com.openfaas.model.IResponse;
import com.openfaas.model.IRequest;
import com.openfaas.model.Response;

import java.lang.Error;
import java.awt.geom.AffineTransform;
import java.awt.image.AffineTransformOp;
import java.net.URL;
import java.awt.image.BufferedImage;
import java.awt.image.WritableRaster;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.awt.image.ColorModel;
import javax.imageio.ImageIO;

//import java.lang.management.ManagementFactory;
//import java.lang.management.MemoryPoolMXBean;

public class Handler implements com.openfaas.model.IHandler {
    static boolean exit; 
    static double scale;
    static BufferedImage image;
    static byte[] binaryImage;
    private int reqCount;

    static {
        try {
            ImageIO.setUseCache(false); // We don't want to cache things out for experimento purposes.

            scale = Double.parseDouble(System.getenv("scale"));
            
            // Reading raw bytes of the image.
            URL url = new URL(System.getenv("image_url"));
            image = ImageIO.read(url);
            int contentLength = url.openConnection().getContentLength();

            ByteArrayOutputStream output = new ByteArrayOutputStream();
            InputStream inputStream = url.openStream();
            int n = 0;
            byte[] buffer = new byte[contentLength];
            while (-1 != (n = inputStream.read(buffer))) {
                output.write(buffer, 0, n);
            }
            
            binaryImage = output.toByteArray();
            int imageSize = binaryImage.length;
            if (imageSize != contentLength) {
                throw new RuntimeException(
                        String.format("Size of the downloaded image %d is different from the content length %d",
                                imageSize, contentLength));
            }
            inputStream.close();
        } catch (Exception e) {
            e.printStackTrace();
            exit = true;
        }
    }

    public IResponse Handle(IRequest req) {
        if (exit) {
            System.exit(1);
        }

        //long edenBefore = getEdenPoolMemUsage();
        long before = System.nanoTime();
        String err = callFunction();
        long after = System.nanoTime();
        //long edenAfter = getEdenPoolMemUsage();
        //System.out.println("EDEN DIFF AFTER CALL FUNC: " + (edenAfter- edenBefore));

        Response res = new Response();
        String output = err + System.lineSeparator();
        if (err.length() == 0) {
            long serviceTime = ((long) (after - before)); // service time in nanoseconds
            output = Long.toString(serviceTime);
        } else {
            res.setStatusCode(500);
        }
        res.setBody(output);
        //System.out.println(this.reqCount + " - HANDLER LEVEL - SERVICE TIME OF callFunction: " + output);
        this.reqCount++;
        return res;
    }

    public String callFunction() {
        String err = "";
        try {
            // avoid that the return from method escape to stack
            byte[] arr = simulateImageDownload();
            
            AffineTransform transform = AffineTransform.getScaleInstance(scale, scale); 
            AffineTransformOp op = new AffineTransformOp(transform, AffineTransformOp.TYPE_BILINEAR); 
            op.filter(image, null).flush();

            // make sure that it will not escape to stack
            for (int i = 0; i < arr.length; i++) {
                arr[i] = binaryImage[i];
            }

        } catch (Exception e) {
            err = e.toString() + System.lineSeparator()
            + e.getCause() + System.lineSeparator()
            + e.getMessage();
            e.printStackTrace();

        } catch (Error e) {
            err = e.toString() + System.lineSeparator()
            + e.getCause() + System.lineSeparator()
            + e.getMessage();
            e.printStackTrace();
        }
        return err;
    }

    private byte[] simulateImageDownload() {
        // This copy aims to simulate the effect of downloading the binary image from an
        // URL, but without having to deal with the variance imposed by network
        // transmission churn.
        byte[] rawCopy = new byte[binaryImage.length];
        for (int i = 0; i < rawCopy.length; i++) {
            rawCopy[i] = binaryImage[i];
        }
        return rawCopy;
    }

    /*private static long getEdenPoolMemUsage() {
        for (final MemoryPoolMXBean pool : ManagementFactory.getMemoryPoolMXBeans()) {
            if (pool.getName().contains("Eden")) {
                return pool.getUsage().getUsed();
            }
        }
        return -1;
    }*/

}