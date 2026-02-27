export default async function handler(req, res) {
  try {
    const query = req.url.replace(/^\/api\/history/, '') || '';
    const target = `${process.env.VITE_API_URL}/diagnose/history${query}`;

    const headers = {
      'Content-Type': 'application/json',
    };

    // forward client X-API-Key if present
    if (req.headers['x-api-key']) {
      headers['X-API-Key'] = req.headers['x-api-key'];
    }

    const response = await fetch(target, { headers });

    // copy status and headers
    res.status(response.status);
    response.headers.forEach((value, key) => {
      // skip hop-by-hop headers
      if (['transfer-encoding','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailers','upgrade'].includes(key.toLowerCase())) return;
      res.setHeader(key, value);
    });

    const body = await response.arrayBuffer();
    res.send(Buffer.from(body));
  } catch (err) {
    console.error('proxy error', err);
    res.status(502).send('proxy error');
  }
}
