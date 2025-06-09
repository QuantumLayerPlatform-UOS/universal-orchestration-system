class CodeGenerator {
  constructor() {
    this.templates = this.loadTemplates();
  }

  loadTemplates() {
    return {
      restAPI: {
        express: this.expressAPITemplate,
        fastify: this.fastifyAPITemplate
      },
      react: {
        functional: this.reactFunctionalComponentTemplate,
        class: this.reactClassComponentTemplate
      },
      database: {
        mongoose: this.mongooseSchemaTemplate,
        sequelize: this.sequelizeModelTemplate
      }
    };
  }

  // Generate REST API endpoint
  generateRestAPI(requirements = {}) {
    const {
      framework = 'express',
      endpoint = '/api/resource',
      methods = ['GET', 'POST', 'PUT', 'DELETE'],
      modelName = 'Resource',
      fields = []
    } = requirements;

    const code = this.expressAPITemplate({
      endpoint,
      methods,
      modelName,
      fields
    });

    return {
      type: 'rest-api',
      framework,
      code,
      files: [{
        name: `${modelName.toLowerCase()}.routes.js`,
        content: code
      }]
    };
  }

  // Generate React component
  generateReactComponent(requirements = {}) {
    const {
      name = 'MyComponent',
      type = 'functional',
      props = [],
      useState = false,
      useEffect = false
    } = requirements;

    const code = type === 'functional' 
      ? this.reactFunctionalComponentTemplate({ name, props, useState, useEffect })
      : this.reactClassComponentTemplate({ name, props });

    return {
      type: 'react-component',
      componentType: type,
      code,
      files: [{
        name: `${name}.jsx`,
        content: code
      }]
    };
  }

  // Generate Express server
  generateExpressServer(requirements = {}) {
    const {
      port = 3000,
      cors = true,
      helmet = true,
      morgan = true,
      routes = []
    } = requirements;

    const code = this.expressServerTemplate({
      port,
      cors,
      helmet,
      morgan,
      routes
    });

    return {
      type: 'express-server',
      code,
      files: [{
        name: 'server.js',
        content: code
      }]
    };
  }

  // Generate database schema
  generateDatabaseSchema(requirements = {}) {
    const {
      orm = 'mongoose',
      modelName = 'Model',
      fields = []
    } = requirements;

    const code = orm === 'mongoose'
      ? this.mongooseSchemaTemplate({ modelName, fields })
      : this.sequelizeModelTemplate({ modelName, fields });

    return {
      type: 'database-schema',
      orm,
      code,
      files: [{
        name: `${modelName.toLowerCase()}.model.js`,
        content: code
      }]
    };
  }

  // Generate code from custom prompt
  generateFromPrompt(requirements = {}) {
    const { prompt = '', language = 'javascript' } = requirements;
    
    // Simple template-based generation based on keywords in prompt
    let code = '';
    
    if (prompt.toLowerCase().includes('function')) {
      code = this.generateFunction(prompt);
    } else if (prompt.toLowerCase().includes('class')) {
      code = this.generateClass(prompt);
    } else if (prompt.toLowerCase().includes('api')) {
      code = this.generateAPIEndpoint(prompt);
    } else {
      code = this.generateGenericCode(prompt);
    }

    return {
      type: 'custom',
      prompt,
      language,
      code,
      files: [{
        name: 'generated-code.js',
        content: code
      }]
    };
  }

  // Template functions
  expressAPITemplate({ endpoint, methods, modelName, fields }) {
    const fieldValidation = fields.map(f => 
      `  ${f.name}: { type: '${f.type}', required: ${f.required || false} }`
    ).join(',\n');

    return `const express = require('express');
const router = express.Router();

// Model import (adjust path as needed)
const ${modelName} = require('../models/${modelName.toLowerCase()}');

${methods.includes('GET') ? `
// GET all ${modelName}s
router.get('${endpoint}', async (req, res) => {
  try {
    const items = await ${modelName}.find();
    res.json(items);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// GET single ${modelName} by ID
router.get('${endpoint}/:id', async (req, res) => {
  try {
    const item = await ${modelName}.findById(req.params.id);
    if (!item) {
      return res.status(404).json({ error: '${modelName} not found' });
    }
    res.json(item);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});` : ''}

${methods.includes('POST') ? `
// POST new ${modelName}
router.post('${endpoint}', async (req, res) => {
  try {
    const newItem = new ${modelName}(req.body);
    const savedItem = await newItem.save();
    res.status(201).json(savedItem);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});` : ''}

${methods.includes('PUT') ? `
// PUT update ${modelName}
router.put('${endpoint}/:id', async (req, res) => {
  try {
    const updatedItem = await ${modelName}.findByIdAndUpdate(
      req.params.id,
      req.body,
      { new: true, runValidators: true }
    );
    if (!updatedItem) {
      return res.status(404).json({ error: '${modelName} not found' });
    }
    res.json(updatedItem);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});` : ''}

${methods.includes('DELETE') ? `
// DELETE ${modelName}
router.delete('${endpoint}/:id', async (req, res) => {
  try {
    const deletedItem = await ${modelName}.findByIdAndDelete(req.params.id);
    if (!deletedItem) {
      return res.status(404).json({ error: '${modelName} not found' });
    }
    res.json({ message: '${modelName} deleted successfully' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});` : ''}

module.exports = router;`;
  }

  reactFunctionalComponentTemplate({ name, props, useState, useEffect }) {
    const propsDestructure = props.length > 0 
      ? `const { ${props.join(', ')} } = props;` 
      : '';

    return `import React${useState ? ', { useState }' : ''}${useEffect ? ', { useEffect }' : ''} from 'react';

const ${name} = (props) => {
  ${propsDestructure}
  ${useState ? `const [state, setState] = useState(null);` : ''}
  
  ${useEffect ? `useEffect(() => {
    // Component mount logic
    console.log('${name} mounted');
    
    return () => {
      // Cleanup logic
      console.log('${name} unmounted');
    };
  }, []);` : ''}

  return (
    <div className="${name.toLowerCase()}-container">
      <h2>${name}</h2>
      ${props.length > 0 ? props.map(p => `<p>{${p}}</p>`).join('\n      ') : '<p>Hello from ${name}!</p>'}
    </div>
  );
};

export default ${name};`;
  }

  reactClassComponentTemplate({ name, props }) {
    return `import React, { Component } from 'react';

class ${name} extends Component {
  constructor(props) {
    super(props);
    this.state = {
      // Initial state
    };
  }

  componentDidMount() {
    console.log('${name} mounted');
  }

  componentWillUnmount() {
    console.log('${name} unmounted');
  }

  render() {
    ${props.length > 0 ? `const { ${props.join(', ')} } = this.props;` : ''}
    
    return (
      <div className="${name.toLowerCase()}-container">
        <h2>${name}</h2>
        ${props.length > 0 ? props.map(p => `<p>{${p}}</p>`).join('\n        ') : '<p>Hello from ${name}!</p>'}
      </div>
    );
  }
}

export default ${name};`;
  }

  expressServerTemplate({ port, cors, helmet, morgan, routes }) {
    return `const express = require('express');
${cors ? "const cors = require('cors');" : ''}
${helmet ? "const helmet = require('helmet');" : ''}
${morgan ? "const morgan = require('morgan');" : ''}

const app = express();
const PORT = process.env.PORT || ${port};

// Middleware
app.use(express.json());
app.use(express.urlencoded({ extended: true }));
${cors ? 'app.use(cors());' : ''}
${helmet ? 'app.use(helmet());' : ''}
${morgan ? "app.use(morgan('dev'));" : ''}

// Routes
${routes.map(r => `app.use('${r.path}', require('./routes/${r.file}'));`).join('\n')}

// Default route
app.get('/', (req, res) => {
  res.json({ message: 'Server is running', status: 'OK' });
});

// Error handling middleware
app.use((err, req, res, next) => {
  console.error(err.stack);
  res.status(500).json({ error: 'Something went wrong!' });
});

// Start server
app.listen(PORT, () => {
  console.log(\`Server is running on port \${PORT}\`);
});

module.exports = app;`;
  }

  mongooseSchemaTemplate({ modelName, fields }) {
    const schemaFields = fields.map(f => {
      const fieldDef = {
        type: this.mapToMongooseType(f.type),
        required: f.required || false
      };
      if (f.default !== undefined) fieldDef.default = f.default;
      if (f.unique) fieldDef.unique = true;
      
      return `  ${f.name}: ${JSON.stringify(fieldDef, null, 2).replace(/"/g, '')}`;
    }).join(',\n');

    return `const mongoose = require('mongoose');

const ${modelName}Schema = new mongoose.Schema({
${schemaFields}
}, {
  timestamps: true
});

// Add any methods or virtuals here
${modelName}Schema.methods.toJSON = function() {
  const obj = this.toObject();
  delete obj.__v;
  return obj;
};

const ${modelName} = mongoose.model('${modelName}', ${modelName}Schema);

module.exports = ${modelName};`;
  }

  sequelizeModelTemplate({ modelName, fields }) {
    const modelFields = fields.map(f => {
      return `    ${f.name}: {
      type: DataTypes.${this.mapToSequelizeType(f.type).toUpperCase()},
      allowNull: ${!f.required},
      ${f.unique ? 'unique: true,' : ''}
      ${f.default !== undefined ? `defaultValue: ${JSON.stringify(f.default)}` : ''}
    }`;
    }).join(',\n');

    return `const { DataTypes } = require('sequelize');

module.exports = (sequelize) => {
  const ${modelName} = sequelize.define('${modelName}', {
${modelFields}
  }, {
    tableName: '${modelName.toLowerCase()}s',
    timestamps: true
  });

  // Define associations here
  ${modelName}.associate = (models) => {
    // Example: ${modelName}.belongsTo(models.User);
  };

  return ${modelName};
};`;
  }

  // Helper methods
  mapToMongooseType(type) {
    const typeMap = {
      'string': 'String',
      'number': 'Number',
      'boolean': 'Boolean',
      'date': 'Date',
      'array': 'Array',
      'object': 'Object'
    };
    return typeMap[type.toLowerCase()] || 'String';
  }

  mapToSequelizeType(type) {
    const typeMap = {
      'string': 'STRING',
      'number': 'INTEGER',
      'boolean': 'BOOLEAN',
      'date': 'DATE',
      'text': 'TEXT',
      'json': 'JSON'
    };
    return typeMap[type.toLowerCase()] || 'STRING';
  }

  generateFunction(prompt) {
    const funcName = this.extractName(prompt, 'function') || 'myFunction';
    return `function ${funcName}(params) {
  // TODO: Implement ${funcName}
  console.log('Function ${funcName} called with:', params);
  
  // Add your logic here
  
  return {
    success: true,
    message: '${funcName} executed successfully'
  };
}

module.exports = ${funcName};`;
  }

  generateClass(prompt) {
    const className = this.extractName(prompt, 'class') || 'MyClass';
    return `class ${className} {
  constructor(options = {}) {
    this.options = options;
    this.initialized = false;
  }

  initialize() {
    // TODO: Add initialization logic
    console.log('${className} initialized');
    this.initialized = true;
  }

  // Add more methods as needed
  process(data) {
    if (!this.initialized) {
      throw new Error('${className} not initialized');
    }
    
    // TODO: Add processing logic
    console.log('Processing data:', data);
    
    return {
      processed: true,
      result: data
    };
  }
}

module.exports = ${className};`;
  }

  generateAPIEndpoint(prompt) {
    const resourceName = this.extractName(prompt, 'api') || 'resource';
    return `// API endpoint for ${resourceName}
router.get('/api/${resourceName}', async (req, res) => {
  try {
    // TODO: Add your API logic here
    const data = {
      message: 'Success',
      ${resourceName}: []
    };
    
    res.json(data);
  } catch (error) {
    console.error('Error in ${resourceName} endpoint:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});`;
  }

  generateGenericCode(prompt) {
    return `// Generated code based on: "${prompt}"
// TODO: Implement the requested functionality

const implementation = () => {
  // Add your implementation here
  console.log('Executing generated code');
  
  return {
    status: 'success',
    message: 'Code executed successfully'
  };
};

module.exports = implementation;`;
  }

  extractName(prompt, type) {
    const words = prompt.split(' ');
    const index = words.findIndex(w => w.toLowerCase().includes(type));
    if (index !== -1 && index + 1 < words.length) {
      return words[index + 1].replace(/[^a-zA-Z0-9]/g, '');
    }
    return null;
  }
}

module.exports = { CodeGenerator };