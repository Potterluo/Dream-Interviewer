package com.yupi.springbootinit.config.redis;

import java.io.*;
import java.util.ArrayList;
import java.util.List;


public class SerializeUtilList {

    /**
     * 序列化 list 集合
     *
     * @param list
     * @return
     */
    public static byte[] serializeList(List<?> list) {
        if (list == null || list.size() == 0) {
            return null;
        }
        ObjectOutputStream oos = null;
        ByteArrayOutputStream baos = null;
        byte[] bytes = null;
        try {
            baos = new ByteArrayOutputStream();
            oos = new ObjectOutputStream(baos);
            for (Object obj : list) {
                oos.writeObject(obj);
            }
            bytes = baos.toByteArray();
        } catch (Exception e) {
            e.printStackTrace();
        } finally {
            close(oos);
            close(baos);
        }
        return bytes;
    }

    /**
     * 反序列化 list 集合
     *
     * @param
     * @return
     */
    public static List<?> unserializeList(byte[] bytes) {
        if (bytes == null) {
            return null;
        }
        List<Object> list = new ArrayList<Object>();
        ByteArrayInputStream bais = null;
        ObjectInputStream ois = null;
        try {
            // 反序列化
            bais = new ByteArrayInputStream(bytes);
            ois = new ObjectInputStream(bais);
            while (bais.available() > 0) {
                Object obj = (Object) ois.readObject();
                if (obj == null) {
                    break;
                }
                list.add(obj);
            }
        } catch (Exception e) {
            e.printStackTrace();
        } finally {
            close(bais);
            close(ois);
        }
        return list;
    }


    public static void close(Closeable closeable) {
        if (closeable != null) {
            try {
                closeable.close();
            } catch (Exception e) {
                e.printStackTrace();
            }
        }
    }

}