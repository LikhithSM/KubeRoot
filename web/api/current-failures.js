export default async function handler(req, res) {
  try {
    const query = req.url.replace(/^\/api\/current-failures/, '') || '';
    const target = `${process.env.VITE_API_URL}/diagnose/current${query}`;

    const headers = {
      'Content-Type': 'application/json',
    };

    if (process.env.VITE_API_KEY) {
      headers['X-API-Key'] = process.env.VITE_API_KEY;
    }
    if (req.headers['x-api-key']) {
      headers['X-API-Key'] = req.headers['x-api-key'];
    }

    const response = await fetch(target, { headers });

    res.status(response.status);
    response.headers.forEach((value, key) => {
      if (['transfer-encoding', 'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailers', 'upgrade'].includes(key.toLowerCase())) return;
      res.setHeader(key, value);
    });

    const body = await response.arrayBuffer();
    res.send(Buffer.from(body));
  } catch (err) {
    console.error('proxy error', err);
    res.status(502).send('proxy error');
  }
}
